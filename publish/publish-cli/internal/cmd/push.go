package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"aly/publish-cli/internal/staging"
	"aly/publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

// outMu 串行化并发上传 goroutine 的进度行输出（审查 M18）
var outMu sync.Mutex

// 推送命令的 flag 变量
var (
	pushVersion          string
	pushMessage          []string
	pushDryRun           bool
	pushForce            bool
	pushScript           string
	pushSetForceUpdate   bool
	pushClearForceUpdate bool
)

func init() {
	cmdPush.Flags().StringVar(&pushVersion, "version", "", "新版本号（必填）")
	cmdPush.Flags().StringArrayVar(&pushMessage, "message", nil, "变更说明（可多次指定）")
	cmdPush.Flags().BoolVar(&pushDryRun, "dry-run", false, "仅校验不实际推送")
	cmdPush.Flags().BoolVar(&pushForce, "force", false, "跳过 MD5 复核强制上传")
	cmdPush.Flags().StringVar(&pushScript, "after-apply-update-script", "", "更新后执行的脚本路径（相对于应用目录）")
	// 审查 B5：force_update 改为三态——
	//   都不传   = 不动服务端现值（旧实现无条件写 false，每次发布都静默关闭强制更新）
	//   --set-*  = 明确打开
	//   --clear-*= 明确关闭
	cmdPush.Flags().BoolVar(&pushSetForceUpdate, "set-force-update", false, "推送后设置此版本为强制更新（客户端不再询问直接更新）")
	cmdPush.Flags().BoolVar(&pushClearForceUpdate, "clear-force-update", false, "推送后取消强制更新（不指定则保持服务端现值不变）")
	cmdPush.MarkFlagRequired("version")

	RootCmd.AddCommand(cmdPush)
}

var cmdPush = &cobra.Command{
	Use:   "push",
	Short: "推送暂存区文件到服务端",
	Run:   runPush,
}

// pushFiles 共享的推送逻辑：并发上传文件，然后创建版本记录
// 返回 true 表示推送成功（可以清理暂存区），false 表示失败或未执行
func pushFiles(cfg RuntimeConfig, version string, messages []string, filesToUpload []staging.StagedFile, isDryRun bool, useForce bool, afterApplyUpdateScript string) bool {
	if len(messages) == 0 {
		if jsonOutput {
			printOutput(false, "至少需要一条 --message", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Error: 至少要一条 --message")
		}
		return false
	}

	client := newAPIClient(cfg)

	if !useForce {
		conflicts, err := staging.Verify(cfg.Path)
		if err != nil {
			outputResult(false, fmt.Sprintf("MD5 校验失败: %v", err), nil)
			return false
		}
		if len(conflicts) > 0 {
			outputResult(false, fmt.Sprintf("以下文件在 add 后被修改，请重新 add: %v", conflicts), nil)
			return false
		}
	}

	if len(filesToUpload) == 0 {
		outputResult(false, "没有要推送的文件", nil)
		return false
	}

	if isDryRun {
		printHumanLn("[DRY RUN] 将上传以下文件：")
		for _, p := range filesToUpload {
			printHumanLn("  %s", p.RelativePath)
		}
		printHumanLn("[DRY RUN] 将创建版本 %s (%d 条日志)", version, len(messages))
		printHumanLn("[DRY RUN] 未实际推送任何内容。")
		return false
	}

	// 阶段 1：并发上传文件
	maxWorkers := runtime.NumCPU()
	if maxWorkers < 3 {
		maxWorkers = 3
	}
	if maxWorkers > 16 {
		maxWorkers = 16
	}
	sem := make(chan struct{}, maxWorkers)
	type uploadResult struct {
		path  string
		index int
		err   error
	}
	uploadCh := make(chan uploadResult, len(filesToUpload))

	// 输出所有文件的 START 进度行（按顺序）
	total := len(filesToUpload)
	for i, f := range filesToUpload {
		printProgress(i+1, total, f.RelativePath, "START", f.LocalSize, "")
	}

	for i, f := range filesToUpload {
		sem <- struct{}{}
		go func(idx int, relPath string, status string, size int64) {
			defer func() {
				if r := recover(); r != nil {
					uploadCh <- uploadResult{path: relPath, index: idx, err: fmt.Errorf("panic: %v", r)}
				}
				<-sem
			}()
			// 审查 M18：每完成一个文件立即输出结果行（加锁避免多 goroutine 交错），
			// 旧实现要等全部上传完才统一输出，GUI 进度条 0%→100% 跳变、上传期间无任何反馈。
			defer func() {}()
			if status == "deleted" {
				if !jsonOutput {
					printHumanLn("Deleting: %s", relPath)
				}
				err := client.DeleteFile(cfg.Shared.ProjectName, relPath)
				outMu.Lock()
				if err != nil {
					printProgressFail(idx+1, total, relPath, size, err.Error())
				} else {
					printProgress(idx+1, total, relPath, "DONE", size, "")
				}
				outMu.Unlock()
				uploadCh <- uploadResult{path: relPath, index: idx, err: err}
				return
			}
			absPath := filepath.Join(cfg.Path, filepath.FromSlash(relPath))
			if !jsonOutput {
				printHumanLn("Uploading: %s", relPath)
			}
			err := client.UploadFile(absPath, cfg.Shared.ProjectName, relPath)
			outMu.Lock()
			if err != nil {
				printProgressFail(idx+1, total, relPath, size, err.Error())
			} else {
				printProgress(idx+1, total, relPath, "DONE", size, "")
			}
			outMu.Unlock()
			uploadCh <- uploadResult{path: relPath, index: idx, err: err}
		}(i, f.RelativePath, f.Status, f.LocalSize)
	}

	// 收集所有上传结果（按原始顺序存储）
	results := make([]uploadResult, len(filesToUpload))
	var firstErr error
	var failedPath string
	for i := 0; i < len(filesToUpload); i++ {
		r := <-uploadCh
		results[r.index] = r
		if r.err != nil && firstErr == nil {
			firstErr = r.err
			failedPath = r.path
		}
	}

	// 审查 M18：结果已在 goroutine 内实时输出，这里只做汇总统计
	_ = results
	if firstErr != nil {
		outputResult(false, fmt.Sprintf("上传失败 [%s]: %v", failedPath, firstErr), nil)
		printProgressDone()
		return false
	}

	// 阶段 2：创建版本记录
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	_, err := client.PublishVersion(models.PublishVersionRequest{
		ProjectName:            cfg.Shared.ProjectName,
		Version:                version,
		Logs:                   messages,
		Time:                   timeStr,
		AfterApplyUpdateScript: afterApplyUpdateScript,
	})
	if err != nil {
		// 审查 B7：旧文案"可重试 push"是误导——
		//   · 若请求确实没到服务端：暂存区可能已被 status 同步移走，需先 add 补回；
		//   · 若已经提交但响应丢失：同版本号重试会被服务端"防倒退保护"拒绝，
		//     而那种拒绝的真实含义是"这个版本其实已经发布成功了"。
		// 所以文案改成先让用户用 log 核对，再决定是否补发。
		outputResult(false, fmt.Sprintf(
			"创建版本失败: %v\n请先执行 aly-publish log 核对版本 %s 是否已发布：\n"+
				"  · 已发布：无需重试（同版本号会被防倒退保护拒绝）\n"+
				"  · 未发布：先 add 缺失文件再重试 push 同版本号", err, version), nil)
		printProgressDone()
		return false
	}

	if jsonOutput {
		printOutput(true, "", map[string]string{
			"version": version,
			"files":   fmt.Sprintf("%d", len(filesToUpload)),
		})
		printProgressDone()
		return true
	}
	printHumanLn("%s published successfully (%d files uploaded)", version, len(filesToUpload))
	return true
}

func setProjectForceUpdate(cfg RuntimeConfig, forceUpdate bool) {
	client := newAPIClient(cfg)
	if err := client.SetProjectForceUpdate(cfg.Shared.ProjectName, forceUpdate); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: 设置强制更新失败: %v\n", err)
	} else if !jsonOutput {
		if forceUpdate {
			printHumanLn("已设置此项目为强制更新模式")
		} else {
			printHumanLn("已取消此项目的强制更新模式")
		}
	}
}

func runPush(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireProject(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	if pushSetForceUpdate && pushClearForceUpdate {
		outputResult(false, "--set-force-update 与 --clear-force-update 不能同时使用", nil)
		return
	}
	if pushDryRun && (pushSetForceUpdate || pushClearForceUpdate) {
		outputResult(false, "--dry-run 不会发布，不能同时指定强制更新开关", nil)
		return
	}

	stagedFiles, loadErr := staging.Load(cfg.Path)
	if loadErr != nil {
		outputResult(false, fmt.Sprintf("读取暂存区失败: %v", loadErr), nil)
		return
	}
	if len(stagedFiles) == 0 {
		outputResult(false, "暂存区为空，请先 add 文件", nil)
		return
	}

	if pushFiles(cfg, pushVersion, pushMessage, stagedFiles, pushDryRun, pushForce, pushScript) {
		if err := staging.Clear(cfg.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: 清理暂存区失败: %v (推送已成功)\n", err)
		}
		// 审查 B5：只有显式传参才触碰服务端 force_update；不传=保持现值。
		// 旧实现无条件 setProjectForceUpdate(false)，导致 GUI 每次发布都静默
		// 关闭强制更新模式（与"编辑项目时特意回传 IsForceUpdate"直接矛盾）。
		if pushSetForceUpdate {
			setProjectForceUpdate(cfg, true)
		} else if pushClearForceUpdate {
			setProjectForceUpdate(cfg, false)
		}
	}
}

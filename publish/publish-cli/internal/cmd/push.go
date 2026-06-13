package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"zap/publish-cli/internal/staging"
	"zap/publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

// 推送命令的 flag 变量
var (
	pushVersion        string
	pushMessage        []string
	pushDryRun         bool
	pushForce          bool
	pushScript         string
	pushSetForceUpdate bool
)

func init() {
	cmdPush.Flags().StringVar(&pushVersion, "version", "", "新版本号（必填）")
	cmdPush.Flags().StringArrayVar(&pushMessage, "message", nil, "变更说明（可多次指定）")
	cmdPush.Flags().BoolVar(&pushDryRun, "dry-run", false, "仅校验不实际推送")
	cmdPush.Flags().BoolVar(&pushForce, "force", false, "跳过 MD5 复核强制上传")
	cmdPush.Flags().StringVar(&pushScript, "after-apply-update-script", "", "更新后执行的脚本路径（相对于应用目录）")
	cmdPush.Flags().BoolVar(&pushSetForceUpdate, "set-force-update", false, "推送后设置此版本为强制更新（客户端不再询问直接更新）")
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
func pushFiles(cfg RuntimeConfig, version string, messages []string, filesToUpload []string, isDryRun bool, useForce bool, afterApplyUpdateScript string) bool {
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
			printHumanLn("  %s", p)
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
		path string
		err  error
	}
	uploadCh := make(chan uploadResult, len(filesToUpload))

	for _, p := range filesToUpload {
		sem <- struct{}{}
		go func(relPath string) {
			defer func() {
				if r := recover(); r != nil {
					uploadCh <- uploadResult{path: relPath, err: fmt.Errorf("panic: %v", r)}
				}
				<-sem
			}()
			absPath := filepath.Join(cfg.Path, filepath.FromSlash(relPath))
			printHumanLn("Uploading: %s", relPath)
			err := client.UploadFile(absPath, cfg.Shared.ProjectName, relPath)
			uploadCh <- uploadResult{path: relPath, err: err}
		}(p)
	}

	var firstErr error
	var failedPath string
	for i := 0; i < len(filesToUpload); i++ {
		r := <-uploadCh
		if r.err != nil && firstErr == nil {
			firstErr = r.err
			failedPath = r.path
		}
	}
	if firstErr != nil {
		outputResult(false, fmt.Sprintf("上传失败 [%s]: %v", failedPath, firstErr), nil)
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
		outputResult(false, fmt.Sprintf("创建版本失败: %v（文件已上传，可重试 push）", err), nil)
		return false
	}

	if jsonOutput {
		printOutput(true, "", map[string]string{
			"version": version,
			"files":   fmt.Sprintf("%d", len(filesToUpload)),
		})
		return true
	}
	printHumanLn("%s published successfully (%d files uploaded)", version, len(filesToUpload))
	return true
}

func setProjectForceUpdate(cfg RuntimeConfig) {
	client := newAPIClient(cfg)
	if err := client.SetProjectForceUpdate(cfg.Shared.ProjectName, true); err != nil {
		printHumanLn("Warning: 设置强制更新失败: %v", err)
	} else {
		printHumanLn("已设置此项目为强制更新模式")
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

	stagedFiles, loadErr := staging.Load(cfg.Path)
	if loadErr != nil {
		outputResult(false, fmt.Sprintf("读取暂存区失败: %v", loadErr), nil)
		return
	}
	if len(stagedFiles) == 0 {
		outputResult(false, "暂存区为空，请先 add 文件", nil)
		return
	}

	var files []string
	for _, f := range stagedFiles {
		files = append(files, f.RelativePath)
	}

	if pushFiles(cfg, pushVersion, pushMessage, files, pushDryRun, pushForce, pushScript) {
		if err := staging.Clear(cfg.Path); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: 清理暂存区失败: %v (推送已成功)\n", err)
		}
		if pushSetForceUpdate {
			setProjectForceUpdate(cfg)
		}
	}
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"publish-cli/internal/config"
	"publish-cli/internal/diff"
	"publish-cli/internal/staging"
	"publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

// 每个命令使用独立的 flag 变量，避免跨命令覆盖
var (
	pushVersion     string
	pushMessage     []string
	pushDryRun      bool
	pushForce       bool

	pushAllVersion  string
	pushAllMessage  []string
	pushAllDryRun   bool
	pushAllForce    bool

	pubVersion      string
	pubMessage      []string
	pubDryRun       bool
)

func init() {
	cmdPush.Flags().StringVar(&pushVersion, "version", "", "新版本号（必填）")
	cmdPush.Flags().StringArrayVar(&pushMessage, "message", nil, "变更说明（可多次指定）")
	cmdPush.Flags().BoolVar(&pushDryRun, "dry-run", false, "仅校验不实际推送")
	cmdPush.Flags().BoolVar(&pushForce, "force", false, "跳过 MD5 复核强制上传")
	cmdPush.MarkFlagRequired("version")

	cmdPushAll.Flags().StringVar(&pushAllVersion, "version", "", "新版本号（必填）")
	cmdPushAll.Flags().StringArrayVar(&pushAllMessage, "message", nil, "变更说明（可多次指定）")
	cmdPushAll.Flags().BoolVar(&pushAllDryRun, "dry-run", false, "仅校验不实际推送")
	cmdPushAll.MarkFlagRequired("version")

	cmdPublish.Flags().StringVar(&pubVersion, "version", "", "新版本号（必填）")
	cmdPublish.Flags().StringArrayVar(&pubMessage, "message", nil, "变更说明（可多次指定）")
	cmdPublish.Flags().BoolVar(&pubDryRun, "dry-run", false, "仅校验不实际推送")
	cmdPublish.MarkFlagRequired("version")

	RootCmd.AddCommand(cmdPush)
	RootCmd.AddCommand(cmdPushAll)
	RootCmd.AddCommand(cmdPublish)
}

var cmdPush = &cobra.Command{
	Use:   "push",
	Short: "推送暂存区文件到服务端",
	Run:   runPush,
}

var cmdPushAll = &cobra.Command{
	Use:   "push-all",
	Short: "一键推送所有变更（无需先 add）",
	Run:   runPushAll,
}

var cmdPublish = &cobra.Command{
	Use:   "publish",
	Short: "完整发布流程（status → add --all → push）",
	Run:   runPublish,
}

// pushFiles 共享的推送逻辑：上传 filesToUpload 指定的文件，然后创建版本记录
// 返回 true 表示推送成功（可以清理暂存区），false 表示失败或未执行
func pushFiles(cfg *config.Config, version string, messages []string, filesToUpload []string, isDryRun bool, useForce bool) bool {
	if len(messages) == 0 {
		if jsonOutput {
			printOutput(false, "至少需要一条 --message", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Error: 至少要一--message")
		}
		return false
	}

	client := newAPIClient(*cfg)
	pid, err := resolveProjectID(*cfg)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return false
	}

	if !useForce {
		conflicts, err := staging.Verify(cfg.Project.Path)
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
		printHumanLn("[DRY RUN] 将创建版 %s (%d 条日", version, len(messages))
		printHumanLn("[DRY RUN] 未实际推送任何内容。")
		return false
	}

	// 阶段 1：逐文件上传
	for _, p := range filesToUpload {
		absPath := filepath.Join(cfg.Project.Path, filepath.FromSlash(p))
		printHumanLn("Uploading: %s", p)
		if err := client.UploadFile(absPath, cfg.Project.Name, p); err != nil {
			outputResult(false, fmt.Sprintf("上传失败 [%s]: %v", p, err), nil)
			return false
		}
	}

	// 阶段 2：创建版本记录
	// NOTE: 文件上传和版本创建是两个独立操作，非原子事务。
	// 如果 PublishVersion 失败，文件已在服务端但无版本记录。
	// 此时应重试 publish 命令（文件已存在则秒传），或手动调用 PublishVersion。
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	_, err = client.PublishVersion(models.PublishVersionRequest{
		ProjectID: pid,
		Version:   version,
		Logs:      messages,
		Time:      timeStr,
	})
	if err != nil {
		outputResult(false, fmt.Sprintf("创建版本失败: %v（文件已上传，可重试 publish 或手动创建版本）", err), nil)
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

	stagedFiles, loadErr := staging.Load(cfg.Project.Path)
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

	if pushFiles(&cfg, pushVersion, pushMessage, files, pushDryRun, pushForce) {
		staging.Clear(cfg.Project.Path)
	}
}

func runPushAll(cmd *cobra.Command, args []string) {
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

	client := newAPIClient(cfg)
	pid, err := resolveProjectID(cfg)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	sd, err := diff.RunStatus(cfg, client, pid)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	var paths []string
	for _, f := range sd.Unstaged {
		if f.Status == "new" || f.Status == "modified" {
			paths = append(paths, f.RelativePath)
		}
	}

	pushFiles(&cfg, pushAllVersion, pushAllMessage, paths, pushAllDryRun, pushAllForce)
}

func runPublish(cmd *cobra.Command, args []string) {
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
	client := newAPIClient(cfg)
	pid, err := resolveProjectID(cfg)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	sd, err := diff.RunStatus(cfg, client, pid)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	var paths []string
	for _, f := range sd.Unstaged {
		if f.Status == "new" || f.Status == "modified" {
			paths = append(paths, f.RelativePath)
		}
	}
	if len(paths) == 0 {
		printHumanLn("没有要发布的文件")
		return
	}
	if err := staging.Add(cfg.Project.Path, paths); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	cfg.Project.ID = pid
	if pushFiles(&cfg, pubVersion, pubMessage, paths, pubDryRun, false) {
		staging.Clear(cfg.Project.Path)
	}
}






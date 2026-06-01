package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"publish-cli/internal/diff"
	"publish-cli/internal/staging"
	"publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

var (
	pushVersion string
	pushMessage []string
	dryRun      bool
	force       bool
)

func init() {
	cmdPush.Flags().StringVar(&pushVersion, "version", "", "新版本号（必填）")
	cmdPush.Flags().StringArrayVar(&pushMessage, "message", nil, "变更说明（可多次指定）")
	cmdPush.Flags().BoolVar(&dryRun, "dry-run", false, "仅校验不实际推送")
	cmdPush.Flags().BoolVar(&force, "force", false, "跳过 MD5 复核强制上传")
	cmdPush.MarkFlagRequired("version")

	cmdPushAll.Flags().StringVar(&pushVersion, "version", "", "新版本号（必填）")
	cmdPushAll.Flags().StringArrayVar(&pushMessage, "message", nil, "变更说明（可多次指定）")
	cmdPushAll.Flags().BoolVar(&dryRun, "dry-run", false, "仅校验不实际推送")
	cmdPushAll.MarkFlagRequired("version")

	cmdPublish.Flags().StringVar(&pushVersion, "version", "", "新版本号（必填）")
	cmdPublish.Flags().StringArrayVar(&pushMessage, "message", nil, "变更说明（可多次指定）")
	cmdPublish.Flags().BoolVar(&dryRun, "dry-run", false, "仅校验不实际推送")
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
	if len(pushMessage) == 0 {
		if jsonOutput {
			printOutput(false, "至少需要一条 --message", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Error: 至少需要一条 --message")
		}
		return
	}

	client := newAPIClient(cfg)
	pid, err := resolveProjectID(cfg)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	if !force {
		conflicts, err := staging.Verify(cfg.Project.Path)
		if err != nil {
			outputResult(false, fmt.Sprintf("MD5 校验失败: %v", err), nil)
			return
		}
		if len(conflicts) > 0 {
			outputResult(false, fmt.Sprintf("以下文件在 add 后被修改，请重新 add: %v", conflicts), nil)
			return
		}
	}

	stagedFiles, _ := staging.Load(cfg.Project.Path)
	if len(stagedFiles) == 0 {
		outputResult(false, "暂存区为空，请先 add 文件", nil)
		return
	}

	if dryRun {
		printHumanLn("[DRY RUN] 将上传以下文件：")
		for _, f := range stagedFiles {
			printHumanLn("  [%s]  %s  %d bytes", f.Status, f.RelativePath, f.LocalSize)
		}
		printHumanLn("[DRY RUN] 将创建版本: %s (%d 条日志)", pushVersion, len(pushMessage))
		printHumanLn("[DRY RUN] 未实际推送任何内容。")
		return
	}

	// 阶段 1：逐文件上传
	for _, f := range stagedFiles {
		absPath := filepath.Join(cfg.Project.Path, filepath.FromSlash(f.RelativePath))
		printHumanLn("Uploading: %s", f.RelativePath)
		if err := client.UploadFile(absPath, cfg.Project.Name, f.RelativePath); err != nil {
			outputResult(false, fmt.Sprintf("上传失败 [%s]: %v", f.RelativePath, err), nil)
			return
		}
	}

	// 阶段 2：创建版本记录
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	_, err = client.PublishVersion(models.PublishVersionRequest{
		ProjectID: pid,
		Version:   pushVersion,
		Logs:      pushMessage,
		Time:      timeStr,
	})
	if err != nil {
		outputResult(false, fmt.Sprintf("创建版本失败: %v", err), nil)
		return
	}

	staging.Clear(cfg.Project.Path)

	if jsonOutput {
		printOutput(true, "", map[string]string{
			"version": pushVersion,
			"files":   fmt.Sprintf("%d", len(stagedFiles)),
		})
		return
	}
	printHumanLn("%s published successfully (%d files uploaded)", pushVersion, len(stagedFiles))
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
	if len(pushMessage) == 0 {
		if jsonOutput {
			printOutput(false, "至少需要一条 --message", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Error: 至少需要一条 --message")
		}
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
		printHumanLn("没有需要推送的文件")
		return
	}

	if dryRun {
		printHumanLn("[DRY RUN] 将上传以下文件：")
		for _, p := range paths {
			printHumanLn("  [%s]  %s", getStatusForPath(sd.Unstaged, p), p)
		}
		printHumanLn("[DRY RUN] 将创建版本: %s (%d 条日志)", pushVersion, len(pushMessage))
		printHumanLn("[DRY RUN] 未实际推送任何内容。")
		return
	}

	// 上传
	for _, p := range paths {
		absPath := filepath.Join(cfg.Project.Path, filepath.FromSlash(p))
		printHumanLn("Uploading: %s", p)
		if err := client.UploadFile(absPath, cfg.Project.Name, p); err != nil {
			outputResult(false, fmt.Sprintf("上传失败 [%s]: %v", p, err), nil)
			return
		}
	}

	// 创建版本
	timeStr := time.Now().Format("2006-01-02 15:04:05")
	_, err = client.PublishVersion(models.PublishVersionRequest{
		ProjectID: pid,
		Version:   pushVersion,
		Logs:      pushMessage,
		Time:      timeStr,
	})
	if err != nil {
		outputResult(false, fmt.Sprintf("创建版本失败: %v", err), nil)
		return
	}

	if jsonOutput {
		printOutput(true, "", map[string]string{
			"version": pushVersion,
			"files":   fmt.Sprintf("%d", len(paths)),
		})
		return
	}
	printHumanLn("%s published successfully (%d files uploaded)", pushVersion, len(paths))
}

func runPublish(cmd *cobra.Command, args []string) {
	// publish = add --all + push
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
		printHumanLn("没有需要发布的文件")
		return
	}
	staging.Add(cfg.Project.Path, paths)
	// 然后 push（runPush 内部会重新 resolveProjectID，缓存命中的是 cfg.Project.ID）
	cfg.Project.ID = pid
	// 重新序列化配置以便 runPush 使用
	runPush(cmd, args)
}

func getStatusForPath(items []models.FileStatusItem, relativePath string) string {
	for i := range items {
		if items[i].RelativePath == relativePath {
			return items[i].Status
		}
	}
	return "unknown"
}

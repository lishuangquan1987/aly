package cmd

import (
	"fmt"

	"aly/publish-cli/internal/diff"
	"aly/publish-cli/internal/staging"
	"aly/publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

var cmdStatus = &cobra.Command{
	Use:   "status",
	Short: "查看本地与服务端文件差异",
	Run:   runStatus,
}

var cmdDiff = &cobra.Command{
	Use:   "diff [--file <relative-path>]",
	Short: "详细对比文件差异",
	Run:   runDiff,
}

var diffFile string

func init() {
	cmdDiff.Flags().StringVar(&diffFile, "file", "", "指定文件相对路径")
	RootCmd.AddCommand(cmdStatus)
	RootCmd.AddCommand(cmdDiff)
}

func runStatus(cmd *cobra.Command, args []string) {
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
	sd, err := diff.RunStatus(cfg.Path, cfg.Shared.IgnoreFolders, cfg.Shared.IgnoreFiles, client, cfg.Shared.ProjectName)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	// 自动同步暂存区：移除"已确认发布成功"的文件
	serverMD5s := make(map[string]string)
	for _, f := range sd.Unstaged {
		if f.ServerMd5 != "" {
			serverMD5s[f.RelativePath] = f.ServerMd5
		}
	}
	for _, f := range sd.Unchanged {
		serverMD5s[f.RelativePath] = f.ServerMd5
	}
	// 审查 B7：加"是否已进入发布清单"第二判据。清单只在 PublishVersion
	// 成功时写入，因此"已上传但版本没建成"的文件会保留在暂存区，用户仍可
	// 重试 push 同版本号。老服务器（无该接口）降级为旧行为。
	var manifestFiles map[string]bool
	if files, err := client.GetReleaseManifest(cfg.Shared.ProjectName); err == nil {
		manifestFiles = make(map[string]bool, len(files))
		for _, f := range files {
			manifestFiles[f] = true
		}
	}
	if removed, err := staging.Sync(cfg.Path, serverMD5s, manifestFiles); err != nil {
		outputResult(false, fmt.Sprintf("同步暂存区失败: %v", err), nil)
		return
	} else if len(removed) > 0 && !jsonOutput {
		printHumanLn("已同步暂存区：移除了 %d 个已上传成功的文件", len(removed))
		for _, f := range removed {
			printHumanLn("        %s", f)
		}
	}

	// 合并暂存区文件：从 unstaged 移除已暂存的，放入 staged
	mergeStagedIntoStatusData(sd, cfg.Path)

	if jsonOutput {
		printOutput(true, "", sd)
		return
	}

	if len(sd.Staged) > 0 {
		printHumanLn("Changes staged for commit:")
		printHumanLn("  (use \"aly-publish reset <file>...\" to unstage)")
		printHumanLn("")
		for _, f := range sd.Staged {
			printHumanLn("        %-12s %s", f.Status+":", f.RelativePath)
		}
		printHumanLn("")
	}
	if len(sd.Unstaged) > 0 {
		printHumanLn("Changes not staged for commit:")
		printHumanLn("  (use \"aly-publish add <file>...\" to stage)")
		printHumanLn("")
		for _, f := range sd.Unstaged {
			printHumanLn("        %-12s %s", f.Status+":", f.RelativePath)
		}
		printHumanLn("")
	}
	if len(sd.Unchanged) > 0 {
		printHumanLn("Unchanged files:")
		for _, f := range sd.Unchanged {
			printHumanLn("        %s", f.RelativePath)
		}
	}
}

func runDiff(cmd *cobra.Command, args []string) {
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
	sd, err := diff.RunStatus(cfg.Path, cfg.Shared.IgnoreFolders, cfg.Shared.IgnoreFiles, client, cfg.Shared.ProjectName)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	// 合并暂存区文件（与 runStatus 一致：从 unstaged 移除已暂存的）
	mergeStagedIntoStatusData(sd, cfg.Path)

	allFiles := make([]models.FileStatusItem, 0, len(sd.Staged)+len(sd.Unstaged)+len(sd.Unchanged))
	allFiles = append(allFiles, sd.Staged...)
	allFiles = append(allFiles, sd.Unstaged...)
	allFiles = append(allFiles, sd.Unchanged...)
	for _, f := range allFiles {
		if diffFile != "" && f.RelativePath != diffFile {
			continue
		}
		printHumanLn("--- %s ---", f.RelativePath)
		printHumanLn("- local:  %s  %s  %d bytes  md5:%s", f.RelativePath, f.Status, f.LocalSize, f.LocalMd5)
		printHumanLn("+ server: %s  %s  %d bytes  md5:%s", f.RelativePath, f.Status, f.ServerSize, f.ServerMd5)
		printHumanLn("")
	}
}

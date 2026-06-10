package cmd

import (
	"zap/publish-cli/internal/diff"
	"zap/publish-cli/internal/staging"
	"zap/publish-cli/pkg/models"

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
	// 合并暂存区文件：从 unstaged 移除已暂存的，放入 staged
	stagedItems := staging.LoadAsStatusItems(cfg.Path)
	stagedPaths := make(map[string]bool, len(stagedItems))
	for _, s := range stagedItems {
		stagedPaths[s.RelativePath] = true
	}
	sd.Staged = stagedItems

	// 从 unstaged 中移除已暂存的文件
	var filteredUnstaged []models.FileStatusItem
	for _, u := range sd.Unstaged {
		if !stagedPaths[u.RelativePath] {
			filteredUnstaged = append(filteredUnstaged, u)
		}
	}
	sd.Unstaged = filteredUnstaged

	if jsonOutput {
		printOutput(true, "", sd)
		return
	}

	if len(sd.Staged) > 0 {
		printHumanLn("Changes staged for commit:")
		printHumanLn("  (use \"publish-cli reset <file>...\" to unstage)")
		printHumanLn("")
		for _, f := range sd.Staged {
			printHumanLn("        %-12s %s", f.Status+":", f.RelativePath)
		}
		printHumanLn("")
	}
	if len(sd.Unstaged) > 0 {
		printHumanLn("Changes not staged for commit:")
		printHumanLn("  (use \"publish-cli add <file>...\" to stage)")
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
	// 合并暂存区文件
	sd.Staged = staging.LoadAsStatusItems(cfg.Path)
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

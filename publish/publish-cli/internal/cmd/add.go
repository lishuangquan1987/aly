package cmd

import (
	"fmt"
	"os"

	"aly/publish-cli/internal/diff"
	"aly/publish-cli/internal/staging"

	"github.com/spf13/cobra"
)

var addAll bool

func init() {
	cmdAdd.Flags().BoolVar(&addAll, "all", false, "添加所有变更文件")
	cmdReset.Flags().BoolVar(&resetAll, "all", false, "清空所有暂存文件")

	RootCmd.AddCommand(cmdAdd)
	RootCmd.AddCommand(cmdReset)
	RootCmd.AddCommand(cmdStaged)
}

var cmdAdd = &cobra.Command{
	Use:   "add [--all | <file>...]",
	Short: "添加文件到暂存区",
	Run:   runAdd,
}

var resetAll bool

var cmdReset = &cobra.Command{
	Use:   "reset [--all | <file>...]",
	Short: "从暂存区移除文件",
	Run:   runReset,
}

var cmdStaged = &cobra.Command{
	Use:   "staged",
	Short: "查看暂存区内容",
	Run:   runStaged,
}

func runAdd(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireProject(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	if addAll {
		if err := requireServer(&cfg); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
		client := newAPIClient(cfg)
		sd, err := diff.RunStatus(cfg.Path, cfg.Shared.IgnoreFolders, cfg.Shared.IgnoreFiles, client, cfg.Shared.ProjectName)
		if err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
		var paths []string
		statusMap := make(map[string]string)
		for _, f := range sd.Unstaged {
			if f.Status == "new" || f.Status == "modified" {
				paths = append(paths, f.RelativePath)
				statusMap[f.RelativePath] = f.Status
			}
		}
		if err := staging.AddWithStatus(cfg.Path, paths, statusMap); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
		if jsonOutput {
			printOutput(true, "", map[string]int{"added": len(paths)})
			return
		}
		printHumanLn("已添加%d 个文件到暂存区", len(paths))
	} else {
		if len(args) == 0 {
			if jsonOutput {
				printOutput(false, "Usage: aly-publish add [--all | <file>...]", nil)
			} else {
				fmt.Fprintln(os.Stderr, "Usage: aly-publish add [--all | <file>...]")
			}
			return
		}
		if err := staging.Add(cfg.Path, args); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
		if jsonOutput {
			printOutput(true, "", map[string]int{"added": len(args)})
			return
		}
		printHumanLn("已添加%d 个文件到暂存区", len(args))
	}
}

func runReset(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireProject(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if resetAll {
		if err := staging.Clear(cfg.Path); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
		if jsonOutput {
			printOutput(true, "", nil)
			return
		}
		printHumanLn("暂存区已清空")
		return
	}
	if len(args) == 0 {
		if jsonOutput {
			printOutput(false, "Usage: aly-publish reset [--all | <file>...]", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Usage: aly-publish reset [--all | <file>...]")
		}
		return
	}
	if err := staging.Remove(cfg.Path, args); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", map[string]int{"removed": len(args)})
		return
	}
	printHumanLn("已从暂存区移除%d 个文件", len(args))
}

func runStaged(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireProject(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	items, err := staging.Load(cfg.Path)
	if err != nil {
		outputResult(false, fmt.Sprintf("failed to load staging: %v", err), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", items)
		return
	}
	if len(items) == 0 {
		printHumanLn("暂存区为空")
		return
	}
	printHumanLn("Staged files:")
	for _, f := range items {
		printHumanLn("  [%s] %s  %d bytes", f.Status, f.RelativePath, f.LocalSize)
	}
}


package cmd

import (
	"fmt"

	"aly/publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

var logLimit int

func init() {
	cmdLog.Flags().IntVar(&logLimit, "limit", 20, "显示条数限制")
	RootCmd.AddCommand(cmdLog)
}

var cmdLog = &cobra.Command{
	Use:   "log",
	Short: "查看版本变更日志",
	Run:   runLog,
}

// validateLogLimit 校验 --limit（审查 M13：负数会 make([]T,-1) panic）
func validateLogLimit(limit int) error {
	if limit <= 0 {
		return fmt.Errorf("invalid --limit: 必须是正整数")
	}
	return nil
}

func runLog(cmd *cobra.Command, args []string) {
	// 审查 M13：--limit 负数会让 make([]T, -1) 直接 panic，0 则静默返回空列表。
	// 负数更像手误（不是"无限制"的约定），统一按 usage 错误处理，行为确定。
	if err := validateLogLimit(logLimit); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	client := newAPIClient(cfg)
	logs, err := client.GetProjectChangeLogs(cfg.Shared.ProjectName)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	// 服务器已按 ID DESC 返回（最新在前），直接截取前 logLimit 条
	count := logLimit
	if count > len(logs) {
		count = len(logs)
	}
	result := make([]models.ProjectChangeLog, count)
	copy(result, logs[:count])

	if jsonOutput {
		printOutput(true, "", result)
		return
	}

	for _, l := range result {
		printHumanLn("%s (%s)", l.Version, l.Time)
		for _, msg := range l.Logs {
			printHumanLn("  • %s", msg)
		}
	}
}

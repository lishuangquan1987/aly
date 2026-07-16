package cmd

import (
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

func runLog(cmd *cobra.Command, args []string) {
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

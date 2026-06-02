package cmd

import (
	"publish-cli/pkg/models"

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
	pid, err := resolveProjectID(cfg)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	logs, err := client.GetProjectChangeLogs(pid)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}

	// 按 ID 倒序排列，统一构建输出列表
	count := logLimit
	if count > len(logs) {
		count = len(logs)
	}
	reversed := make([]models.ProjectChangeLog, 0, count)
	for i := len(logs) - 1; i >= 0 && len(reversed) < logLimit; i-- {
		reversed = append(reversed, logs[i])
	}

	if jsonOutput {
		printOutput(true, "", reversed)
		return
	}

	for _, l := range reversed {
		printHumanLn("%s (%s)", l.Version, l.Time)
		for _, msg := range l.Logs {
			printHumanLn("  • %s", msg)
		}
	}
}

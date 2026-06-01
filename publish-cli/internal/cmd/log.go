package cmd

import (
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
	// 按 ID 倒序排列
	for i := len(logs) - 1; i >= 0; i-- {
		if len(logs)-1-i >= logLimit {
			break
		}
		l := logs[i]
		if jsonOutput {
			continue // JSON 一次性输出
		}
		printHumanLn("%s (%s)", l.Version, l.Time)
		for _, msg := range l.Logs {
			printHumanLn("  • %s", msg)
		}
	}
	if jsonOutput {
		// 倒序输出
		reversed := make([]interface{}, 0)
		for i := len(logs) - 1; i >= 0; i-- {
			if len(logs)-1-i >= logLimit {
				break
			}
			reversed = append(reversed, logs[i])
		}
		printOutput(true, "", reversed)
	}
}

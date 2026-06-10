package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(cmdServer)
	cmdServer.AddCommand(cmdServerInfo)
}

var cmdServer = &cobra.Command{
	Use:   "server",
	Short: "服务器信息",
}

var cmdServerInfo = &cobra.Command{
	Use:   "info",
	Short: "查看服务端系统信息",
	Run:   runServerInfo,
}

func runServerInfo(cmd *cobra.Command, args []string) {
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
	info, err := client.GetServerInfo()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", info)
		return
	}
	if len(info) == 0 {
		printHumanLn("无服务器信息")
		return
	}
	s := info[0]
	printHumanLn("OS:              %s", s.OS)
	printHumanLn("Platform:        %s", s.Platform)
	printHumanLn("Architecture:    %s", s.GOARCH)
	printHumanLn("Go Version:      %s", s.Version)
	printHumanLn("CPU:             %s", s.CPUName)
	printHumanLn("Cores:           %d @ %.0f MHz", s.NumCPU, s.CPUMhz)
	printHumanLn("Disk:            %.1f GiB used / %.1f GiB total (%.1f%%)", s.DiskUsed, s.DiskTotal, s.DiskUsedPercent)
}

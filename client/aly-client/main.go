package main

import (
	"fmt"
	"os"

	"aly/client/aly-client/cmd"
	"aly/client/aly-client/config"
	"aly/client/aly-client/util"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// 宿主生命周期（规范见 spec/host-contract.md §15）：宿主退出后普通命令跟随退出；
	// 例外是 apply_update 阶段——命令级 apply_update，或 version.json 的
	// version_status == applying（崩溃恢复窗口），两者取或。
	util.WatchHostExit(command == "apply_update", applyingNow, func() {
		fmt.Fprintf(os.Stderr, "host process exited; updater exits (command=%s)\n", command)
	})

	switch command {
	case "check_update":
		cmd.CheckUpdate()
	case "check_diff":
		cmd.CheckDiff()
	case "download_update":
		cmd.DownloadUpdate()
	case "apply_update":
		cmd.ApplyUpdate()
	case "list_rollback_versions":
		cmd.ListRollbackVersions()
	case "rollback":
		cmd.Rollback()
	case "clean":
		cmd.Clean()
	case "check_self_update":
		cmd.CheckSelfUpdate()
	case "self_update":
		cmd.SelfUpdate()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

// applyingNow 读工作目录（UpdateFolder/）的 version.json，判断是否处在 apply 例外窗口。
// 读不到 / 解析失败都按 false 处理：状态不明时不做例外，宿主退出后照常跟随退出。
func applyingNow() bool {
	vi, err := config.ReadVersion()
	if err != nil {
		return false
	}
	return vi.VersionStatus == config.VersionStatusApplying
}

func printUsage() {
	fmt.Println("用法: aly-client <命令> [选项]")
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  check_update              检查更新")
	fmt.Println("  check_diff                文件比对")
	fmt.Println("  download_update           下载更新")
	fmt.Println("  apply_update              执行更新")
	fmt.Println("  list_rollback_versions    列出可回滚版本")
	fmt.Println("  rollback                  版本回滚")
	fmt.Println("  clean                     清理历史版本快照与临时文件")
	fmt.Println("  check_self_update         自更新检查")
	fmt.Println("  self_update               用主程序目录的新版更新器替换自身")
	fmt.Println()
	fmt.Println("使用 -h 查看各命令的选项")
}

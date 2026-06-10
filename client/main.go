package main

import (
	"fmt"
	"os"

	"zap/client/cmd"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
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
	case "check_self_update":
		cmd.CheckSelfUpdate()
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("用法: zap-update <命令> [选项]")
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  check_update              检查更新")
	fmt.Println("  check_diff                文件比对")
	fmt.Println("  download_update           下载更新")
	fmt.Println("  apply_update              执行更新")
	fmt.Println("  list_rollback_versions    列出可回滚版本")
	fmt.Println("  rollback                  版本回滚")
	fmt.Println("  check_self_update         自更新检查")
	fmt.Println()
	fmt.Println("使用 -h 查看各命令的选项")
}

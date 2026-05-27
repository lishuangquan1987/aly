package cmd

import (
	"flag"
	"fmt"
	"os"

	"clientupdator/client/config"
)

// ListRollbackVersions 列出可回滚版本
func ListRollbackVersions() {
	fs := flag.NewFlagSet("list_rollback_versions", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "主程序相对路径")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, "")

	updateDir, err := cfg.UpdateDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取更新目录失败: %v\n", err)
		return
	}

	// 检查 update 目录是否存在
	info, err := os.Stat(updateDir)
	if err != nil || !info.IsDir() {
		// update 目录不存在，没有可回滚的版本
		return
	}

	// 读取 update 目录下的子目录
	dir, err := os.Open(updateDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "打开更新目录失败: %v\n", err)
		return
	}
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取更新目录失败: %v\n", err)
		return
	}

	// 读取当前版本信息
	versionInfo, err := config.ReadVersion()
	currentVersion := ""
	if err == nil {
		currentVersion = versionInfo.Version
	}

	for _, name := range names {
		// 跳过当前版本（当前版本不需要回滚）
		if name == currentVersion {
			continue
		}
		subDirPath := updateDir + string(os.PathSeparator) + name
		subInfo, err := os.Stat(subDirPath)
		if err == nil && subInfo.IsDir() {
			fmt.Println(name)
		}
	}
}

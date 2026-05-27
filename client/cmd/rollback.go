package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"clientupdator/client/config"
	"clientupdator/client/util"
)

// Rollback 版本回滚
func Rollback() {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	versionFlag := fs.String("version", "", "回滚目标版本号")
	mainExePathFlag := fs.String("main-exe-path", "", "主程序相对路径")
	mustCloseFlag := fs.String("must-close-process-name", "", "必须关闭的进程名（逗号分隔）")
	fs.Parse(os.Args[2:])

	if *versionFlag == "" {
		fmt.Println("false:请指定回滚目标版本号 (--version)")
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, *mustCloseFlag)

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}

	// 检查回滚版本目录是否存在
	rollbackDir := filepath.Join(mainFolder, "update", *versionFlag)
	info, err := os.Stat(rollbackDir)
	if err != nil || !info.IsDir() {
		fmt.Printf("false:回滚版本 %s 不存在\n", *versionFlag)
		return
	}

	// 1. 关闭必须关闭的进程
	if len(cfg.MustCloseProcessName) > 0 {
		fmt.Fprintf(os.Stderr, "正在关闭进程...\n")
		if err := util.KillProcessesAndWait(cfg.MustCloseProcessName, 10*time.Second); err != nil {
			fmt.Printf("false:关闭进程失败: %v\n", err)
			return
		}
	}

	// 2. 将回滚版本目录中的文件复制到主程序目录
	fmt.Fprintf(os.Stderr, "从 %s 复制文件到 %s\n", rollbackDir, mainFolder)
	if err := util.CopyDir(rollbackDir, mainFolder, true); err != nil {
		fmt.Printf("false:复制回滚文件失败: %v\n", err)
		return
	}

	// 3. 更新 version.json
	versionInfo, err := config.ReadVersion()
	if err != nil {
		fmt.Printf("false:读取版本信息失败: %v\n", err)
		return
	}
	versionInfo.Version = *versionFlag
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		fmt.Printf("false:更新版本信息失败: %v\n", err)
		return
	}

	fmt.Println("true")
}

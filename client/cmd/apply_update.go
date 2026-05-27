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

// ApplyUpdate 执行更新
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "主程序相对路径")
	mustCloseFlag := fs.String("must-close-process-name", "", "必须关闭的进程名（逗号分隔）")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, *mustCloseFlag)

	// 1. 读取 version.json，检查状态
	versionInfo, err := config.ReadVersion()
	if err != nil {
		fmt.Printf("false:读取版本信息失败: %v\n", err)
		return
	}

	if versionInfo.VersionStatus != config.VersionStatusDownloaded {
		fmt.Println("false:当前没有待应用的更新")
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}

	// 2. 关闭必须关闭的进程
	if len(cfg.MustCloseProcessName) > 0 {
		fmt.Fprintf(os.Stderr, "正在关闭进程...\n")
		if err := util.KillProcessesAndWait(cfg.MustCloseProcessName, 10*time.Second); err != nil {
			fmt.Printf("false:关闭进程失败: %v\n", err)
			return
		}
	}

	// 3. 将 update/{version}/ 中的文件复制到主程序目录
	versionDir := filepath.Join(mainFolder, "update", versionInfo.Version)
	fmt.Fprintf(os.Stderr, "从 %s 复制文件到 %s\n", versionDir, mainFolder)

	if err := util.CopyDir(versionDir, mainFolder, true); err != nil {
		fmt.Printf("false:复制更新文件失败: %v\n", err)
		return
	}

	// 4. 更新 version.json
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		fmt.Printf("false:更新版本信息失败: %v\n", err)
		return
	}

	fmt.Println("true")
}

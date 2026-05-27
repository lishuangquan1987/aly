package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"clientupdator/client/config"
	"clientupdator/client/model"
)

// ApplyUpdate 执行更新
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	mustCloseFlag := fs.String("must-close-process-name", "", "process names to close (comma separated)")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds for process close")
	silentFlag := fs.Bool("silent", false, "silent mode")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("load config: %v", err)})
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, *mustCloseFlag)

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("read version: %v", err)})
		return
	}
	if versionInfo.VersionStatus != config.VersionStatusDownloaded {
		printJSON(model.SuccessOutput{Success: false, Error: "no pending update to apply"})
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: err.Error()})
		return
	}

	// 关闭必须关闭的进程
	if len(cfg.MustCloseProcessName) > 0 {
		if !*silentFlag {
			fmt.Fprintf(os.Stderr, "closing processes...\n")
		}
		closeProcessesGracefully(cfg.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// 原子替换
	if !*silentFlag {
		fmt.Fprintf(os.Stderr, "applying update...\n")
	}
	if err := atomicReplace(mainFolder, versionInfo.Version); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: err.Error()})
		return
	}

	// 更新 version.json
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("write version: %v", err)})
		return
	}

	// 执行更新后脚本
	if cfg.PostUpdateScript != "" {
		scriptPath := cfg.PostUpdateScript
		// 如果脚本路径是相对路径，则相对于 ExeDir 解析
		if len(scriptPath) > 0 && (scriptPath[0] == '.' || (len(scriptPath) >= 2 && scriptPath[1] != ':')) {
			exeDir, _ := config.ExeDir()
			scriptPath = exeDir + string(os.PathSeparator) + scriptPath
		}
		runScript(scriptPath)
	}

	// 启动主程序
	launchMainExe(cfg)

	printJSON(model.SuccessOutput{Success: true})
}

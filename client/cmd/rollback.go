package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"clientupdator/client/config"
	"clientupdator/client/model"
)

// Rollback reverts to a previous version
func Rollback() {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	versionFlag := fs.String("version", "", "target version to rollback to")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	mustCloseFlag := fs.String("must-close-process-name", "", "process names to close")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds")
	fs.Parse(os.Args[2:])

	if *versionFlag == "" {
		printJSON(model.SuccessOutput{Success: false, Error: "--version is required"})
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("load config: %v", err)})
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, *mustCloseFlag)

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: err.Error()})
		return
	}

	rollbackDir := filepath.Join(mainFolder, "update", *versionFlag)
	if info, err := os.Stat(rollbackDir); err != nil || !info.IsDir() {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("version %s not found", *versionFlag)})
		return
	}

	// Close processes
	if len(cfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(cfg.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// Atomic replace
	if err := atomicReplace(mainFolder, *versionFlag); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: err.Error()})
		return
	}

	// Update version.json
	versionInfo, err := config.ReadVersion()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("read version: %v", err)})
		return
	}
	versionInfo.Version = *versionFlag
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("write version: %v", err)})
		return
	}

	// Launch main exe
	launchMainExe(cfg)

	printJSON(model.SuccessOutput{Success: true, Version: *versionFlag})
}

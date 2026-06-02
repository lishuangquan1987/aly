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

// Rollback reverts to a previous version (same procedure as apply_update)
func Rollback() {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	versionFlag := fs.String("version", "", "target version to rollback to")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	mustCloseFlag := fs.String("must-close-process-name", "", "process names to close")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds")
	fs.Parse(os.Args[2:])

	if *versionFlag == "" {
		printOutput(false, "--version is required", nil)
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		printOutput(false, fmt.Sprintf("load config: %v", err), nil)
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, *mustCloseFlag)

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	versionDir, err := cfg.AppVersionDir(*versionFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	if info, statErr := os.Stat(versionDir); statErr != nil || !info.IsDir() {
		printOutput(false, fmt.Sprintf("version %s not found", *versionFlag), nil)
		return
	}

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}
	oldVersion := versionInfo.Version

	// Check version_status for crash recovery
	switch versionInfo.VersionStatus {
	case config.VersionStatusApplying:
		// Crash recovery
		if _, statErr := os.Stat(mainFolder); statErr == nil {
			// Main folder exists, redo replacement steps (fall through)
		} else {
			// Main folder doesn't exist, check if target version dir exists
			if _, statErr := os.Stat(versionDir); statErr == nil {
				if err := os.Rename(versionDir, mainFolder); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				versionInfo.Version = *versionFlag
				versionInfo.VersionPrevious = oldVersion
				versionInfo.VersionStatus = config.VersionStatusApplied
				config.WriteVersion(versionInfo)
				if cfg.PostUpdateScript != "" {
					runScript(filepath.Join(mainFolder, cfg.PostUpdateScript))
				}
				launchMainExe(cfg)
				printOutput(true, "", nil)
				return
			}
			printOutput(false, "crash recovery failed: neither main folder nor target version folder exists", nil)
			return
		}
	}

	// Set version_status = "applying" (mark start of rollback)
	versionInfo.VersionStatus = config.VersionStatusApplying
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Close processes gracefully
	if len(cfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(cfg.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// Rollback target version dir already has complete files from when it was active.
	// Unlike apply_update (which needs CopyDirWithExclude to fill in unchanged files
	// from the current folder), rollback only needs atomic rename.

	// Compute paths for atomic rename
	prevVersionDir, err := cfg.AppVersionDir(oldVersion)
	if err != nil {
		versionInfo.VersionStatus = config.VersionStatusApplied
		config.WriteVersion(versionInfo)
		printOutput(false, err.Error(), nil)
		return
	}
	// Temporarily move old backup aside instead of deleting upfront (safer for power failure)
	oldBackupTemp := prevVersionDir + ".old"
	os.RemoveAll(oldBackupTemp)
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		os.Rename(prevVersionDir, oldBackupTemp)
	}

	// Rename mainFolder -> prevVersionDir (backup current)
	if err := os.Rename(mainFolder, prevVersionDir); err != nil {
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			os.Rename(oldBackupTemp, prevVersionDir)
		}
		versionInfo.VersionStatus = config.VersionStatusApplied
		config.WriteVersion(versionInfo)
		printOutput(false, fmt.Sprintf("backup rename failed: %v", err), nil)
		return
	}

	// Rename versionDir -> mainFolder (activate rollback target)
	if err := os.Rename(versionDir, mainFolder); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		os.Rename(prevVersionDir, mainFolder)
		os.Rename(oldBackupTemp, prevVersionDir)
		versionInfo.VersionStatus = config.VersionStatusApplied
		config.WriteVersion(versionInfo)
		printOutput(false, fmt.Sprintf("apply rename failed: %v", err), nil)
		return
	}

	// Clean up old backup AFTER successful rename
	os.RemoveAll(oldBackupTemp)

	// Update version.json
	versionInfo.VersionPrevious = oldVersion
	versionInfo.Version = *versionFlag
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Run post_update_script if configured
	if cfg.PostUpdateScript != "" {
		runScript(filepath.Join(mainFolder, cfg.PostUpdateScript))
	}

	// Launch main exe
	launchMainExe(cfg)

	printOutput(true, "", &model.RollbackData{Version: *versionFlag})
}

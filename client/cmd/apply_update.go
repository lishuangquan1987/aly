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

// ApplyUpdate applies a downloaded update with atomic replacement
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	mustCloseFlag := fs.String("must-close-process-name", "", "process names to close (comma separated)")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds for process close")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		printOutput(false, fmt.Sprintf("load config: %v", err), nil)
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, *mustCloseFlag)

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// Check version_status
	switch versionInfo.VersionStatus {
	case config.VersionStatusApplied:
		printOutput(false, "no pending update to apply", nil)
		return

	case config.VersionStatusApplying:
		// Crash recovery
		if _, statErr := os.Stat(mainFolder); statErr == nil {
			// Main folder exists and is complete -> redo replacement steps (fall through)
		} else {
			// Main folder doesn't exist, check if version dir exists
			versionDir, verDirErr := cfg.AppVersionDir(versionInfo.Version)
			if verDirErr != nil {
				printOutput(false, verDirErr.Error(), nil)
				return
			}
			if _, statErr := os.Stat(versionDir); statErr == nil {
				// Rename AppVersionDir to MainExeFolderPath
				if err := os.Rename(versionDir, mainFolder); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				// Update status to applied
				versionInfo.VersionStatus = config.VersionStatusApplied
				config.WriteVersion(versionInfo)
				// Run post-update script and launch main exe
				if cfg.PostUpdateScript != "" {
					runScript(filepath.Join(mainFolder, cfg.PostUpdateScript))
				}
				launchMainExe(cfg)
				printOutput(true, "", nil)
				return
			}
			printOutput(false, "crash recovery failed: neither main folder nor version folder exists", nil)
			return
		}

	case config.VersionStatusDownloaded:
		// Normal flow, continue
	}

	// Set version_status = "applying"
	versionInfo.VersionStatus = config.VersionStatusApplying
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Close processes gracefully
	if len(cfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(cfg.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// Atomic replacement
	versionDir, err := cfg.AppVersionDir(versionInfo.Version)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// Step 6c: Copy current mainFolder content to versionDir (excluding un_copy_files and un_copy_folders)
	// Ensures versionDir is a complete runnable app
	if err := util.CopyDirWithExclude(mainFolder, versionDir, cfg.ShouldSkipFile, cfg.ShouldSkipFolder); err != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		config.WriteVersion(versionInfo)
		printOutput(false, fmt.Sprintf("copy to version dir: %v", err), nil)
		return
	}

	// Step 6d: Compute paths for atomic rename
	prevVersionDir, err := cfg.AppVersionDir(versionInfo.VersionPrevious)
	if err != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
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

	// Step 6e: Rename mainFolder -> prevVersionDir (backup)
	if err := os.Rename(mainFolder, prevVersionDir); err != nil {
		// Restore old backup if it existed
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			os.Rename(oldBackupTemp, prevVersionDir)
		}
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		config.WriteVersion(versionInfo)
		printOutput(false, fmt.Sprintf("backup rename failed: %v", err), nil)
		return
	}

	// Step 6f: Rename versionDir -> mainFolder
	if err := os.Rename(versionDir, mainFolder); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		os.Rename(prevVersionDir, mainFolder)
		os.Rename(oldBackupTemp, prevVersionDir)
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		config.WriteVersion(versionInfo)
		printOutput(false, fmt.Sprintf("apply rename failed: %v", err), nil)
		return
	}

	// Clean up old backup AFTER successful rename
	os.RemoveAll(oldBackupTemp)

	// Step 7: Update version.json
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Step 8: Run post_update_script if configured
	if cfg.PostUpdateScript != "" {
		runScript(filepath.Join(mainFolder, cfg.PostUpdateScript))
	}

	// Step 9: Launch main exe
	launchMainExe(cfg)

	// Step 10: Output success
	printOutput(true, "", nil)
}

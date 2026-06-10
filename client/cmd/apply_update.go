package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"zap/client/config"
	"zap/client/util"
)

// ApplyUpdate applies a downloaded update with atomic replacement
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	mustCloseFlag := fs.String("must-close-process-name", "", "process names to close (comma separated)")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds for process close")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig("", "", *mainExePathFlag, *mustCloseFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}

	// Check version_status
	switch versionInfo.VersionStatus {
	case config.VersionStatusApplied:
		printOutput(false, "no pending update to apply", nil)
		return

	case config.VersionStatusApplying:
		// Crash recovery
		if _, statErr := os.Stat(fc.MainFolder); statErr == nil {
			// Main folder exists and is complete -> redo replacement steps (fall through)
		} else {
			// Main folder doesn't exist, check if version dir exists
			versionDir, verDirErr := fc.ExeCfg.AppVersionDir(versionInfo.Version)
			if verDirErr != nil {
				printOutput(false, verDirErr.Error(), nil)
				return
			}
			if _, statErr := os.Stat(versionDir); statErr == nil {
				// Rename AppVersionDir to MainExeFolderPath
				if err := os.Rename(versionDir, fc.MainFolder); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				// Update status to applied
				versionInfo.VersionStatus = config.VersionStatusApplied
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(".", "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				// Run post-update script and launch main exe
				if fc.Client.PostUpdateScript != "" {
					runScript(filepath.Join(fc.MainFolder, fc.Client.PostUpdateScript))
				}
				launchMainExe(fc.ExeCfg)
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
	if len(fc.Client.MustCloseProcessName) > 0 {
		closeProcessesGracefully(fc.Client.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// Atomic replacement
	versionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.Version)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// Copy current mainFolder content to versionDir (excluding ignore folders/files from shared.json)
	shouldSkipFile := func(relPath string) bool {
		return config.ShouldSkipFile(relPath, fc.Shared.IgnoreFiles)
	}
	shouldSkipFolder := func(relPath string) bool {
		return config.ShouldSkipFolder(relPath, fc.Shared.IgnoreFolders)
	}
	if err := util.CopyDirWithExclude(fc.MainFolder, versionDir, shouldSkipFile, shouldSkipFolder); err != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after copy fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("copy to version dir: %v", err), nil)
		return
	}

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.VersionPrevious)
	if err != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after prevVersionDir err: write version failed: %v", wErr))
		}
		printOutput(false, err.Error(), nil)
		return
	}
	// Temporarily move old backup aside instead of deleting upfront (safer for power failure)
	oldBackupTemp := prevVersionDir + ".old"
	os.RemoveAll(oldBackupTemp)
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		os.Rename(prevVersionDir, oldBackupTemp)
	}

	// Rename mainFolder -> prevVersionDir (backup)
	if err := os.Rename(fc.MainFolder, prevVersionDir); err != nil {
		// Restore old backup if it existed
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			os.Rename(oldBackupTemp, prevVersionDir)
		}
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after backup rename fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("backup rename failed: %v", err), nil)
		return
	}

	// Rename versionDir -> mainFolder
	if err := os.Rename(versionDir, fc.MainFolder); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		os.Rename(prevVersionDir, fc.MainFolder)
		os.Rename(oldBackupTemp, prevVersionDir)
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after apply rename fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("apply rename failed: %v", err), nil)
		return
	}

	// Clean up old backup AFTER successful rename
	os.RemoveAll(oldBackupTemp)

	// Update version.json
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Run post_update_script if configured
	if fc.Client.PostUpdateScript != "" {
		runScript(filepath.Join(fc.MainFolder, fc.Client.PostUpdateScript))
	}

	// Launch main exe
	launchMainExe(fc.ExeCfg)

	// Output success
	printOutput(true, "", nil)
}

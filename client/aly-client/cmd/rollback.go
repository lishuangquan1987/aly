package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

// Rollback reverts to a previous version (same procedure as apply_update)
func Rollback() {
	fs := flag.NewFlagSet("rollback", flag.ExitOnError)
	versionFlag := fs.String("version", "", "target version to rollback to")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds")
	fs.Parse(os.Args[2:])

	if *versionFlag == "" {
		printOutput(false, "--version is required", nil)
		return
	}

	fc, err := loadFullConfig("", "", *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	versionDir, err := fc.ExeCfg.AppVersionDir(*versionFlag)
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
		if _, statErr := os.Stat(fc.MainFolder); statErr == nil {
			// Main folder exists, redo replacement steps (fall through)
		} else {
			// Main folder doesn't exist, check if target version dir exists
			if _, statErr := os.Stat(versionDir); statErr == nil {
				if err := os.Rename(versionDir, fc.MainFolder); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				versionInfo.Version = *versionFlag
				versionInfo.VersionPrevious = oldVersion
				versionInfo.VersionStatus = config.VersionStatusApplied
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(".", "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				if versionInfo.AfterApplyUpdateScript != "" {
					runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript))
				}
				launchMainExe(fc.ExeCfg)
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
	if len(fc.ExeCfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// Rollback target version dir already has complete files from when it was active.
	// Unlike apply_update (which needs CopyDirWithExclude to fill in unchanged files
	// from the current folder), rollback only needs atomic rename.

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(oldVersion)
	if err != nil {
		versionInfo.VersionStatus = config.VersionStatusApplied
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after prevVersionDir err: write version failed: %v", wErr))
		}
		printOutput(false, err.Error(), nil)
		return
	}
	// Temporarily move old backup aside instead of deleting upfront (safer for power failure)
	oldBackupTemp := prevVersionDir + ".old"
	if err := os.RemoveAll(oldBackupTemp); err != nil {
		util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: remove old backup temp failed: %v", err))
	}
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		if err := os.Rename(prevVersionDir, oldBackupTemp); err != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: rename prevVersionDir to oldBackupTemp failed: %v", err))
		}
	}

	// Rename mainFolder -> prevVersionDir (backup current)
	if err := os.Rename(fc.MainFolder, prevVersionDir); err != nil {
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			if rErr := os.Rename(oldBackupTemp, prevVersionDir); rErr != nil {
				util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: restore oldBackupTemp to prevVersionDir failed: %v", rErr))
			}
		}
		versionInfo.VersionStatus = config.VersionStatusApplied
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after backup rename fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("backup rename failed: %v", err), nil)
		return
	}

	// Rename versionDir -> mainFolder (activate rollback target)
	if err := os.Rename(versionDir, fc.MainFolder); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		if rErr := os.Rename(prevVersionDir, fc.MainFolder); rErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: restore prevVersionDir to mainFolder failed: %v", rErr))
		}
		if rErr := os.Rename(oldBackupTemp, prevVersionDir); rErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: restore oldBackupTemp to prevVersionDir failed: %v", rErr))
		}
		versionInfo.VersionStatus = config.VersionStatusApplied
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after apply rename fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("apply rename failed: %v", err), nil)
		return
	}

	// Clean up old backup AFTER successful rename
	if err := os.RemoveAll(oldBackupTemp); err != nil {
		util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: cleanup oldBackupTemp failed: %v", err))
	}

	// Update version.json
	versionInfo.VersionPrevious = oldVersion
	versionInfo.Version = *versionFlag
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Run post-update script if configured
	if versionInfo.AfterApplyUpdateScript != "" {
		runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript))
	}

	// Launch main exe
	launchMainExe(fc.ExeCfg)

	printOutput(true, "", &model.RollbackData{Version: *versionFlag})
}

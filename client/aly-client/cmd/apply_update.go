package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aly/client/aly-client/config"
	"aly/client/aly-client/util"
)

// logDir 返回日志目录，ExeDir 失败时 fallback 到当前目录
func logDir() string {
	dir, err := config.ExeDir()
	if err != nil || dir == "" {
		return "."
	}
	return dir
}

// ApplyUpdate applies a downloaded update with atomic replacement
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds for process close")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig("", "", *mainExePathFlag)
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
					util.AppendToLog(logDir(), "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				// Run post-update script and launch main exe
				if versionInfo.AfterApplyUpdateScript != "" {
					runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript))
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
	if len(fc.ExeCfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, time.Duration(*closeTimeoutFlag)*time.Second)
	}

	// Atomic replacement
	versionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.Version)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// 从 versionDir 读取 shared.json，获取 un_copy_folders / un_copy_files
	// 这些字段指定不应从当前 ApplicationFolder 复制到新版本目录的文件/文件夹
	var unCopyFolders []string
	var unCopyFiles []string
	if versionShared, verErr := config.LoadSharedConfig(versionDir); verErr == nil {
		unCopyFolders = versionShared.UnCopyFolders
		unCopyFiles = versionShared.UnCopyFiles
	}

	// Copy current mainFolder content to versionDir.
	// 只排除 un_copy_folders / un_copy_files（仅用于 apply-update 时的复制控制）。
	// ignore_folders / ignore_files 用于服务端文件列表过滤和 publish-cli 文件采集，不在此处使用。
	shouldSkipFile := func(relPath string) bool {
		return config.ShouldSkipFile(relPath, unCopyFiles)
	}
	shouldSkipFolder := func(relPath string) bool {
		return config.ShouldSkipFolder(relPath, unCopyFolders)
	}
	if err := util.CopyDirWithExclude(fc.MainFolder, versionDir, shouldSkipFile, shouldSkipFolder); err != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("rollback after copy fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("copy to version dir: %v", err), nil)
		return
	}

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.VersionPrevious)
	if err != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("rollback after prevVersionDir err: write version failed: %v", wErr))
		}
		printOutput(false, err.Error(), nil)
		return
	}
	// Temporarily move old backup aside instead of deleting upfront (safer for power failure)
	oldBackupTemp := prevVersionDir + ".old"
	if err := os.RemoveAll(oldBackupTemp); err != nil {
		exeDir := logDir()
		util.AppendToLog(exeDir, "update.log", fmt.Sprintf("remove old backup temp: %v", err))
	}
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		if err := os.Rename(prevVersionDir, oldBackupTemp); err != nil {
			exeDir := logDir()
			util.AppendToLog(exeDir, "update.log", fmt.Sprintf("backup rename to temp: %v", err))
			// Continue anyway — the main rename will fail and trigger rollback
		}
	}

	// Rename mainFolder -> prevVersionDir (backup)
	if err := os.Rename(fc.MainFolder, prevVersionDir); err != nil {
		// Restore old backup if it existed
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			if rerr := os.Rename(oldBackupTemp, prevVersionDir); rerr != nil {
				exeDir := logDir()
				util.AppendToLog(exeDir, "update.log", fmt.Sprintf("rollback restore backup: %v", rerr))
			}
		}
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("rollback after backup rename fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("backup rename failed: %v", err), nil)
		return
	}

	// Rename versionDir -> mainFolder
	if err := os.Rename(versionDir, fc.MainFolder); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		if rerr := os.Rename(prevVersionDir, fc.MainFolder); rerr != nil {
			exeDir := logDir()
			util.AppendToLog(exeDir, "update.log", fmt.Sprintf("rollback main rename: %v", rerr))
		}
		if rerr := os.Rename(oldBackupTemp, prevVersionDir); rerr != nil {
			exeDir := logDir()
			util.AppendToLog(exeDir, "update.log", fmt.Sprintf("rollback backup restore: %v", rerr))
		}
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("rollback after apply rename fail: write version failed: %v", wErr))
		}
		printOutput(false, fmt.Sprintf("apply rename failed: %v", err), nil)
		return
	}

	// Clean up old backup AFTER successful rename
	if err := os.RemoveAll(oldBackupTemp); err != nil {
		exeDir := logDir()
		util.AppendToLog(exeDir, "update.log", fmt.Sprintf("cleanup old backup: %v", err))
	}

	// Update version.json
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

	// Output success
	printOutput(true, "", nil)
}

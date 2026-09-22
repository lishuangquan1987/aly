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

	closeTimeout := time.Duration(*closeTimeoutFlag) * time.Second

	// 全局更新锁：同一时刻只允许一个更新操作（下载/应用/回滚）
	releaseLock, lockErr := AcquireUpdateLock("rollback")
	if lockErr != nil {
		printOutput(false, lockErr.Error(), nil)
		return
	}
	defer releaseLock()

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

	// 守卫：downloaded 状态下不允许回滚到"待应用的下载版本"。
	// 此时 versionInfo.Version 是待应用的新版本，其目录（ApplicationFolder_{Version}）
	// 是下载产物而非可回滚的快照，回滚会导致版本状态错乱。
	if versionInfo.VersionStatus == config.VersionStatusDownloaded && *versionFlag == versionInfo.Version {
		printOutput(false, fmt.Sprintf("版本 %s 是未应用的下载版本，不可回滚", *versionFlag), nil)
		return
	}

	// 备份目录名必须取"替换前 MainFolder 的真实版本"：
	// downloaded 状态下 MainFolder 里仍是 VersionPrevious 的内容（Version 只是待应用的新版本），
	// 若用 versionInfo.Version 命名备份，会出现"目录名 V2 装着 V1 内容"的错配，
	// 并把待下载目录（ApplicationFolder_{Version}）误当备份旁移后删除。
	// 仅当 status=applying（崩溃恢复）时信任持久化的 RollbackPrevious，
	// 避免陈旧字段在后续普通回滚中被误用。
	oldVersion := versionInfo.Version
	if versionInfo.VersionStatus == config.VersionStatusDownloaded && versionInfo.VersionPrevious != "" {
		oldVersion = versionInfo.VersionPrevious
	}
	if versionInfo.VersionStatus == config.VersionStatusApplying && versionInfo.RollbackPrevious != "" {
		oldVersion = versionInfo.RollbackPrevious
	}

	// 守卫：回滚到"当前活动版本"是无操作（该场景仅当活动版本目录残留存在时可达，
	// 否则会在上面的 versionDir stat 检查处被拦截），给出明确提示而非走替换流程。
	if *versionFlag == oldVersion {
		printOutput(false, fmt.Sprintf("当前已处于版本 %s，无需回滚", *versionFlag), nil)
		return
	}

	// Check version_status for crash recovery
	switch versionInfo.VersionStatus {
	case config.VersionStatusApplying:
		// Crash recovery
		if _, statErr := os.Stat(fc.MainFolder); statErr == nil {
			// Main folder exists, redo replacement steps (fall through)
		} else {
			// Main folder doesn't exist, check if target version dir exists
			if _, statErr := os.Stat(versionDir); statErr == nil {
				if err := renameDirWithKill(versionDir, fc.MainFolder, closeTimeout); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				versionInfo.Version = *versionFlag
				versionInfo.VersionPrevious = oldVersion
				versionInfo.VersionStatus = config.VersionStatusApplied
				versionInfo.RollbackPrevious = ""
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(".", "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				if versionInfo.AfterApplyUpdateScript != "" {
					runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript), fc.MainFolder)
				}
				launchMainExe(fc.ExeCfg, fc.MainFolder)
				printOutput(true, "", nil)
				return
			}
			printOutput(false, "crash recovery failed: neither main folder nor target version folder exists", nil)
			return
		}
	}

	// Set version_status = "applying" (mark start of rollback)，
	// 并持久化本次回滚的 pre-rollback active version，供崩溃恢复使用。
	versionInfo.VersionStatus = config.VersionStatusApplying
	versionInfo.RollbackPrevious = oldVersion
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Close processes gracefully
	if len(fc.ExeCfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
	}

	// 共享探测状态：本次 rollback 内 3 次 rename 复用同一探测/击杀结果，避免重复全量扫描（#23）。
	st := newRenameProbeState()

	// Rollback target version dir already has complete files from when it was active.
	// Unlike apply_update (which needs CopyDirWithExclude to fill in unchanged files
	// from the current folder), rollback only needs atomic rename.

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(oldVersion)
	if err != nil {
		versionInfo.VersionStatus = config.VersionStatusApplied
		versionInfo.RollbackPrevious = ""
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
		// 旧备份目录存在：必须先挪开，否则主目录重命名会因目标非空目录报 Access denied
		if err := renameDirWithKillState(prevVersionDir, oldBackupTemp, closeTimeout, st); err != nil {
			versionInfo.VersionStatus = config.VersionStatusApplied
			versionInfo.RollbackPrevious = ""
			if wErr := config.WriteVersion(versionInfo); wErr != nil {
				util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after backup aside fail: write version failed: %v", wErr))
			}
			printOutput(false, fmt.Sprintf("backup aside failed: %v", err), nil)
			return
		}
	}

	// Rename mainFolder -> prevVersionDir (backup current)
	if err := renameDirWithKillState(fc.MainFolder, prevVersionDir, closeTimeout, st); err != nil {
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			if rErr := os.Rename(oldBackupTemp, prevVersionDir); rErr != nil {
				util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: restore oldBackupTemp to prevVersionDir failed: %v", rErr))
			}
		}
		versionInfo.VersionStatus = config.VersionStatusApplied
		versionInfo.RollbackPrevious = ""
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after backup rename fail: write version failed: %v", wErr))
		}
		// 回滚失败：启动当前版本主程序（保持应用可用），并附带错误信息
		launchMainExeFn(fc.ExeCfg, fc.MainFolder)
		printOutput(false, fmt.Sprintf("backup rename failed: %v", err), nil)
		return
	}

	// Rename versionDir -> mainFolder (activate rollback target)
	if err := renameDirWithKillState(versionDir, fc.MainFolder, closeTimeout, st); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		if rErr := os.Rename(prevVersionDir, fc.MainFolder); rErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: restore prevVersionDir to mainFolder failed: %v", rErr))
		}
		if rErr := os.Rename(oldBackupTemp, prevVersionDir); rErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback: restore oldBackupTemp to prevVersionDir failed: %v", rErr))
		}
		versionInfo.VersionStatus = config.VersionStatusApplied
		versionInfo.RollbackPrevious = ""
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after apply rename fail: write version failed: %v", wErr))
		}
		// 回滚失败：启动当前版本主程序（保持应用可用），并附带错误信息
		launchMainExeFn(fc.ExeCfg, fc.MainFolder)
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
	versionInfo.RollbackPrevious = ""
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// Run post-update script if configured
	if versionInfo.AfterApplyUpdateScript != "" {
		runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript), fc.MainFolder)
	}

	// Launch main exe
	launchMainExe(fc.ExeCfg, fc.MainFolder)

	printOutput(true, "", &model.RollbackData{Version: *versionFlag})
}

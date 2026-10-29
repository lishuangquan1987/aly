package cmd

import (
	"flag"
	"fmt"
	"os"
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
	mustCloseFlag := fs.String("must-close-process-name", "", "comma separated process names to close")
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
	// C# SDK 会传 --must-close-process-name（逗号分隔），合并进配置（#1）
	mergeMustCloseFlag(fc, *mustCloseFlag)

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}

	// 机会式清理历史版本快照（保留最近 N 个，Bug#6）。
	// 额外保护 CLI 目标：入口清理不得删掉用户正要回滚到的旧快照。
	pruneVersionSnapshots(fc, defaultSnapshotKeep, *versionFlag)

	// 未知 version_status：不猜测、不替换（与 apply_update 口径一致，F6）。
	// 只有 download_update 能把它重写成 downloaded 完成修复。
	switch versionInfo.VersionStatus {
	case "", config.VersionStatusApplied, config.VersionStatusDownloaded, config.VersionStatusApplying:
		// 已知状态：继续（空状态与 applied 同义，仅用于"激活历史快照"这种无歧义场景）
	default:
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("rollback 拒绝：未知 version_status %q（不替换）", versionInfo.VersionStatus))
		printOutput(false, fmt.Sprintf("unknown version_status %q, refuse to rollback", versionInfo.VersionStatus), nil)
		return
	}

	// 升级中断现场（status=applying 且 rollback_previous 为空）= 一次**未完成的 apply**：
	// 其"备份目录名"必须取 apply 的语义（Version），而回滚要用 VersionPrevious/真实内容版本，
	// 二者冲突——直接走下面的替换会把待应用的 App_{Version}（下载好的升级包）旁移后删除，
	// 并把 MainFolder 的旧内容改名成 App_{Version}（目录名与内容错配，违反回滚判据），
	// 之后回滚到该版本会装入错内容（F9）。故明确拒绝，提示先让在途 apply 完成恢复。
	if versionInfo.VersionStatus == config.VersionStatusApplying && versionInfo.RollbackPrevious == "" {
		util.AppendToLog(logDir(), "update.log",
			"rollback 拒绝：有一次未完成的 apply（applying 且无 rollback_previous），请先完成 apply_update 恢复")
		printOutput(false, "有一次未完成的更新（applying），请先执行 apply_update 完成恢复后再回滚", nil)
		return
	}

	// 回滚中断现场（status=applying && rollback_previous != ""）：目标目录可能已被消耗
	// （激活已完成），先委托崩溃恢复续跑，不再以 "version not found" 拒绝（修复 Bug#1 B2'）。
	// 持久化的 RollbackTarget 优先，老数据（无该字段）用 CLI --version 兜底。
	if versionInfo.VersionStatus == config.VersionStatusApplying && versionInfo.RollbackPrevious != "" {
		if err := resumeRollback(fc, versionInfo, closeTimeout, *versionFlag); err != nil {
			printOutput(false, fmt.Sprintf("rollback crash recovery failed: %v", err), nil)
			return
		}
		resumed := versionInfo.RollbackTarget
		if resumed == "" {
			resumed = *versionFlag
		}
		printOutput(true, "", &model.RollbackData{Version: resumed})
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

	// 回滚前的状态：任何失败路径都必须恢复它，而不是一律写 applied（F8）——
	// 在 downloaded 状态下回滚失败时，写 applied 会让 version.json 声称 Version 已生效，
	// 而磁盘内容其实是 VersionPrevious（状态与内容错配，且此后无法自愈）。
	preStatus := versionInfo.VersionStatus

	// Check version_status for crash recovery
	// 注：下面的 applying 分支在当前两道入口守卫（RP!="" 委托 resumeRollback、RP=="" 直接拒绝）
	// 之后已不可达，保留为防御性恢复路径（守卫若被改动仍能自愈）。
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
				clearUpdateTransientState(versionInfo)
				versionInfo.VersionStatus = config.VersionStatusApplied
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(".", "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				cleanupOrphanBackupIfUnreferenced(fc, versionInfo)
				runPostApplyScript(fc, versionInfo.AfterApplyUpdateScript, versionInfo.Version)
				launchMainExe(fc.ExeCfg, fc.MainFolder)
				printOutput(true, "", nil)
				return
			}
			printOutput(false, "crash recovery failed: neither main folder nor target version folder exists", nil)
			return
		}
	}

	// Set version_status = "applying" (mark start of rollback)，
	// 并持久化本次回滚的 pre-rollback active version 与回滚目标版本，供崩溃恢复使用。
	versionInfo.VersionStatus = config.VersionStatusApplying
	versionInfo.RollbackPrevious = oldVersion
	versionInfo.RollbackTarget = *versionFlag
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
	// Unlike download_update (which materializes unchanged files into the pending
	// version dir), rollback only needs atomic rename.

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(oldVersion)
	if err != nil {
		// F8：失败恢复回滚前状态（不是一律 applied）
		clearUpdateTransientState(versionInfo)
		versionInfo.VersionStatus = preStatus
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after prevVersionDir err: write version failed: %v", wErr))
		}
		printOutput(false, err.Error(), nil)
		return
	}
	// 加固①：不再在入口无条件清理 .old 残留（崩溃点"旁移后/备份改名前"时该残留是
	// 尚未被本次替换覆盖的历史快照），清理时机统一后移到新备份就位后的成功路径。
	oldBackupTemp := prevVersionDir + ".old"
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		// 旧备份目录存在：必须先挪开，否则主目录重命名会因目标非空目录报 Access denied
		if err := renameDirWithKillState(prevVersionDir, oldBackupTemp, closeTimeout, st); err != nil {
			// F8：失败恢复回滚前状态（不是一律 applied）
			clearUpdateTransientState(versionInfo)
			versionInfo.VersionStatus = preStatus
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
		// F8：失败恢复回滚前状态（不是一律 applied）
		clearUpdateTransientState(versionInfo)
		versionInfo.VersionStatus = preStatus
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
		// F8：失败恢复回滚前状态（不是一律 applied）
		clearUpdateTransientState(versionInfo)
		versionInfo.VersionStatus = preStatus
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("rollback after apply rename fail: write version failed: %v", wErr))
		}
		// 回滚失败：启动当前版本主程序（保持应用可用），并附带错误信息
		launchMainExeFn(fc.ExeCfg, fc.MainFolder)
		printOutput(false, fmt.Sprintf("apply rename failed: %v", err), nil)
		return
	}

	// Clean up old backup AND any aside variants AFTER successful rename（#18）
	removeAsideVariants(prevVersionDir)

	// Update version.json
	versionInfo.VersionPrevious = oldVersion
	versionInfo.Version = *versionFlag
	versionInfo.VersionStatus = config.VersionStatusApplied
	clearUpdateTransientState(versionInfo)
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}
	// F2：目标版本激活后，"无版本号备份目录"在 Previous 为空时已不再被引用 → 清理垃圾备份
	cleanupOrphanBackupIfUnreferenced(fc, versionInfo)

	// Run post-update script if configured
	runPostApplyScript(fc, versionInfo.AfterApplyUpdateScript, versionInfo.Version)

	// Launch main exe
	launchMainExe(fc.ExeCfg, fc.MainFolder)

	printOutput(true, "", &model.RollbackData{Version: *versionFlag})
}

// resumeRollback 完成一次被中断的回滚（version.json: status=applying && rollback_previous != ""）。
// 按磁盘现场 4 分支恢复，只做重命名（不复制文件），任何一步失败返回 error（状态保持 applying
// 由上层重试）；成功则补写 applied 并启动主程序。绝不把回滚当成升级处理（修复 Bug#1/#5）。
//
// cliTarget 仅用于老数据（无 rollback_target 字段）时的目标兜底；apply_update 委托时传 ""，
// 此时若仍无 rollback_target，则按"安全放弃回滚"处理（恢复/归位到回滚前版本，不升级）。
func resumeRollback(fc *FullConfig, vi *config.VersionInfo, closeTimeout time.Duration, cliTarget string) error {
	oldVersion := vi.RollbackPrevious
	target := vi.RollbackTarget
	if target == "" {
		target = cliTarget
	}

	// 无法确定目标版本：安全放弃回滚 —— 若备份还在则恢复，随后归位到回滚前版本。
	if target == "" {
		if _, statErr := os.Stat(fc.MainFolder); os.IsNotExist(statErr) {
			if prevDir, err := fc.ExeCfg.AppVersionDir(oldVersion); err == nil {
				if _, statErr2 := os.Stat(prevDir); statErr2 == nil {
					closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
					if err := renameDirWithKill(prevDir, fc.MainFolder, closeTimeout); err != nil {
						return fmt.Errorf("restore rollback backup: %v", err)
					}
				}
			}
		}
		vi.Version = oldVersion
		vi.VersionPrevious = oldVersion
		vi.VersionStatus = config.VersionStatusApplied
		vi.RollbackPrevious = ""
		vi.RollbackTarget = ""
		return finishRollbackRecovery(fc, vi)
	}

	targetDir, err := fc.ExeCfg.AppVersionDir(target)
	if err != nil {
		return err
	}
	prevDir, err := fc.ExeCfg.AppVersionDir(oldVersion)
	if err != nil {
		return err
	}

	if _, statErr := os.Stat(fc.MainFolder); statErr == nil {
		// MainFolder 存在
		if _, statErr2 := os.Stat(targetDir); os.IsNotExist(statErr2) {
			// 分支 1：激活已完成（目标目录已被消耗）→ 补写 applied
			vi.Version = target
			vi.VersionPrevious = oldVersion
			vi.VersionStatus = config.VersionStatusApplied
			vi.RollbackPrevious = ""
			vi.RollbackTarget = ""
			return finishRollbackRecovery(fc, vi)
		}
		// 分支 2：备份改名前的崩溃 → 重做回滚替换（仅重命名）
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
		st := newRenameProbeState()
		removeAsideVariants(prevDir)
		oldBackupTemp := prevDir + ".old"
		if _, statErr3 := os.Stat(prevDir); statErr3 == nil {
			if err := renameDirWithKillState(prevDir, oldBackupTemp, closeTimeout, st); err != nil {
				return fmt.Errorf("backup aside failed: %v", err)
			}
		}
		if err := renameDirWithKillState(fc.MainFolder, prevDir, closeTimeout, st); err != nil {
			// 恢复被旁移的旧备份
			if _, statErr3 := os.Stat(oldBackupTemp); statErr3 == nil {
				os.Rename(oldBackupTemp, prevDir)
			}
			return fmt.Errorf("backup rename failed: %v", err)
		}
		if err := renameDirWithKillState(targetDir, fc.MainFolder, closeTimeout, st); err != nil {
			// 回滚主目录
			os.Rename(prevDir, fc.MainFolder)
			if _, statErr3 := os.Stat(oldBackupTemp); statErr3 == nil {
				os.Rename(oldBackupTemp, prevDir)
			}
			return fmt.Errorf("apply rename failed: %v", err)
		}
		removeAsideVariants(prevDir)
		vi.Version = target
		vi.VersionPrevious = oldVersion
		vi.VersionStatus = config.VersionStatusApplied
		vi.RollbackPrevious = ""
		vi.RollbackTarget = ""
		return finishRollbackRecovery(fc, vi)
	}

	// MainFolder 缺失
	if _, statErr := os.Stat(targetDir); statErr == nil {
		// 分支 3：备份改名后、激活前崩溃 → 激活目标
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
		if err := renameDirWithKill(targetDir, fc.MainFolder, closeTimeout); err != nil {
			return fmt.Errorf("rollback crash recovery rename: %v", err)
		}
		vi.Version = target
		vi.VersionPrevious = oldVersion
		vi.VersionStatus = config.VersionStatusApplied
		vi.RollbackPrevious = ""
		vi.RollbackTarget = ""
		return finishRollbackRecovery(fc, vi)
	}
	if _, statErr := os.Stat(prevDir); statErr == nil {
		// 分支 4：目标已消耗（B2' 现场）→ 放弃回滚，恢复回滚前版本
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
		if err := renameDirWithKill(prevDir, fc.MainFolder, closeTimeout); err != nil {
			return fmt.Errorf("restore rollback backup: %v", err)
		}
		vi.Version = oldVersion
		vi.VersionPrevious = oldVersion
		vi.VersionStatus = config.VersionStatusApplied
		vi.RollbackPrevious = ""
		vi.RollbackTarget = ""
		return finishRollbackRecovery(fc, vi)
	}
	return fmt.Errorf("rollback crash recovery failed: neither main folder, target dir, nor backup exists")
}

// finishRollbackRecovery 回滚崩溃恢复成功后收尾：写 applied 状态 + 后置脚本 + 启动主程序。
func finishRollbackRecovery(fc *FullConfig, vi *config.VersionInfo) error {
	// 离开回滚现场：清空 RollbackPrevious/RollbackTarget（防 prune 永久保护陈旧目标，F7）
	// 与 apply 阶段标记（F5）
	clearUpdateTransientState(vi)
	if err := config.WriteVersion(vi); err != nil {
		util.AppendToLog(logDir(), "update.log", fmt.Sprintf("rollback crash recovery: write version failed: %v", err))
		return fmt.Errorf("write version: %v", err)
	}
	// F2：无版本号备份目录在 Previous 为空时不再被引用 → 清理
	cleanupOrphanBackupIfUnreferenced(fc, vi)
	runPostApplyScript(fc, vi.AfterApplyUpdateScript, vi.Version)
	launchMainExe(fc.ExeCfg, fc.MainFolder)
	return nil
}

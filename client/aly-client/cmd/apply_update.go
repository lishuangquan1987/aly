package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
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

// mergeMustCloseFlag 将 --must-close-process-name（逗号分隔）解析并合并进配置。
// C# SDK（AlyApi.cs）会显式传该参数，而 Go 端此前未定义此 flag 导致 os.Exit(2)（#1）。
func mergeMustCloseFlag(fc *FullConfig, flagValue string) {
	if flagValue == "" {
		return
	}
	names := strings.Split(flagValue, ",")
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		fc.ExeCfg.MustCloseProcessName = append(fc.ExeCfg.MustCloseProcessName, n)
	}
}

// ApplyUpdate applies a downloaded update with atomic replacement
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds for process close")
	mustCloseFlag := fs.String("must-close-process-name", "", "comma separated process names to close")
	fs.Parse(os.Args[2:])

	closeTimeout := time.Duration(*closeTimeoutFlag) * time.Second

	// 全局更新锁：同一时刻只允许一个更新操作（下载/应用/回滚）
	releaseLock, lockErr := AcquireUpdateLock("apply_update")
	if lockErr != nil {
		printOutput(false, lockErr.Error(), nil)
		return
	}
	defer releaseLock()

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

	// 机会式清理历史版本快照（保留最近 N 个，Bug#6）
	pruneVersionSnapshots(fc, defaultSnapshotKeep)

	// Check version_status
	switch versionInfo.VersionStatus {
	case config.VersionStatusApplied, "":
		// applied 与"空状态"（version.json 缺失 / 损坏被隔离后重置 / 首次部署）同义：
		// 都没有"已下载待应用"的版本。空状态此前会 fall through 到替换流程，出现
		// "把 MainFolder 改名到自己"的假成功（{P:"",V:"",S:""} → applied，F1）。
		printOutput(false, "no pending update to apply", nil)
		return

	case config.VersionStatusApplying:
		// 回滚中断（status=applying && rollback_previous != ""）：按回滚语义恢复，
		// 绝不按 versionInfo.Version 升级（修复 Bug#1/#5）。apply_update 是 SDK 宿主
		// check→apply 循环中实际被调用的命令，在此委托即可让回滚自动续跑（无需改 SDK）。
		if versionInfo.RollbackPrevious != "" {
			if err := resumeRollback(fc, versionInfo, closeTimeout, ""); err != nil {
				printOutput(false, fmt.Sprintf("rollback crash recovery failed: %v", err), nil)
				return
			}
			printOutput(true, "", nil)
			return
		}
		// Crash recovery
		if _, statErr := os.Stat(fc.MainFolder); statErr == nil {
			// Main folder exists —— 需要区分两种情况：
			//   1. versionDir 已不存在：说明 versionDir→mainFolder 重命名已完成，
			//      新版本已就位，只差写 applied 状态。此时绝不能再跑替换流程——
			//      重跑会把"上一版本"备份（prevVersionDir）连同旧备份一起删掉，
			//      回滚永久失效。
			//   2. versionDir 仍存在：崩溃发生在复制/重命名之前，mainFolder 还是旧版本，
			//      走正常替换流程（fall through）。
			versionDir, verDirErr := fc.ExeCfg.AppVersionDir(versionInfo.Version)
			if verDirErr != nil {
				printOutput(false, verDirErr.Error(), nil)
				return
			}
			if _, statErr2 := os.Stat(versionDir); os.IsNotExist(statErr2) {
				// 新版本已应用完成：补齐状态、执行后置脚本、启动主程序。
				if versionInfo.ApplyStage != config.ApplyStageActivating {
					// F5：缺少"激活改名已开始"的证据（老版本写的状态 / 状态被手工改坏）。
					// 仍按历史启发式判定为已完成（兼容老现场），但必须留痕，便于事后核对
					// "磁盘内容与写入的版本号是否真的匹配"。
					util.AppendToLog(logDir(), "update.log",
						"apply crash recovery: versionDir 缺失但缺少 apply_stage=activating 证据，按历史启发式判定为已完成")
				}
				clearUpdateTransientState(versionInfo)
				versionInfo.VersionStatus = config.VersionStatusApplied
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(logDir(), "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				cleanupOrphanBackupIfUnreferenced(fc, versionInfo)
				runPostApplyScript(fc, versionInfo.AfterApplyUpdateScript, versionInfo.Version)
				launchMainExe(fc.ExeCfg, fc.MainFolder)
				printOutput(true, "", nil)
				return
			}
			// versionDir 存在：崩溃发生在替换前，重做替换步骤（fall through）
		} else {
			// Main folder doesn't exist, check if version dir exists
			versionDir, verDirErr := fc.ExeCfg.AppVersionDir(versionInfo.Version)
			if verDirErr != nil {
				printOutput(false, verDirErr.Error(), nil)
				return
			}
			if _, statErr := os.Stat(versionDir); statErr == nil {
				// Rename AppVersionDir to MainExeFolderPath
				if err := renameDirWithKill(versionDir, fc.MainFolder, closeTimeout); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				// Update status to applied
				clearUpdateTransientState(versionInfo)
				versionInfo.VersionStatus = config.VersionStatusApplied
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(logDir(), "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				cleanupOrphanBackupIfUnreferenced(fc, versionInfo)
				// Run post-update script and launch main exe
				runPostApplyScript(fc, versionInfo.AfterApplyUpdateScript, versionInfo.Version)
				launchMainExe(fc.ExeCfg, fc.MainFolder)
				printOutput(true, "", nil)
				return
			}
			printOutput(false, "crash recovery failed: neither main folder nor version folder exists", nil)
			return
		}

	case config.VersionStatusDownloaded:
		// 正常流程。替换前先把"待应用现场"校验清楚（F3/F4/A1）：
		// 拒绝的分支都在写 applying 之前返回 —— 不写状态、不关进程、不改名。
		if versionInfo.Version == "" {
			// A1：downloaded 但 Version 为空属异常/遗留状态：AppVersionDir("") 会退化成
			// 无版本号备份目录（ApplicationFolder_），把它当待应用版本会装入陈旧内容或
			// 做一次"改名到自己"的假成功。与 F3 一致自愈为 applied + 真实内容版本。
			util.AppendToLog(logDir(), "update.log",
				"apply 拒绝：downloaded 但 Version 为空（异常/遗留状态，不替换）")
			clearUpdateTransientState(versionInfo)
			versionInfo.Version = versionInfo.VersionPrevious
			versionInfo.VersionStatus = config.VersionStatusApplied
			if wErr := config.WriteVersion(versionInfo); wErr != nil {
				util.AppendToLog(logDir(), "update.log", fmt.Sprintf("repair write version failed: %v", wErr))
			}
			printOutput(false, "版本状态异常：Version 为空，已回退为 applied（原本就没有可应用的版本号）", nil)
			return
		}
		if versionInfo.VersionPrevious != "" && versionInfo.VersionPrevious == versionInfo.Version {
			// 异常/遗留状态：Version == VersionPrevious 时备份目录与版本目录同路径，
			// 旁移-备份-激活会自毁。应用层提前拒绝（applyReplacement 里还有一道守卫），
			// 避免"拒绝 → 兜底回 downloaded → check 仍要求 apply"的反复杀进程循环（F4）。
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("apply 拒绝：Version == VersionPrevious(%s)（异常/遗留状态，不替换）", versionInfo.Version))
			printOutput(false, fmt.Sprintf("版本状态异常：Version == VersionPrevious(%s)，拒绝替换", versionInfo.Version), nil)
			return
		}
		pendingDir, verErr := fc.ExeCfg.AppVersionDir(versionInfo.Version)
		if verErr != nil {
			printOutput(false, verErr.Error(), nil)
			return
		}
		pendingCount, countErr := countRegularFiles(pendingDir)
		if countErr != nil && !os.IsNotExist(countErr) {
			// A2：目录存在但读不到（ACL/杀软锁/慢盘超时）：不得据此自愈丢弃可能有效的待应用包
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("apply 拒绝：待应用版本目录不可读 %s: %v", pendingDir, countErr))
			printOutput(false, fmt.Sprintf("待应用版本目录不可读: %s（%v），未做任何改动", pendingDir, countErr), nil)
			return
		}
		incomplete := countErr != nil /*目录缺失*/ || pendingCount == 0 ||
			(versionInfo.DownloadedFileCount > 0 && pendingCount < versionInfo.DownloadedFileCount)
		if incomplete {
			// 下载产物缺失/为空/文件数少于下载完成时记录的数量：此前会把 MainFolder 自拷贝进
			// 版本目录后整体改名，报"更新成功"但内容其实没变（或混装旧/新文件的半成品）。
			// 这里拒绝替换，并把状态自愈成 applied + 真实内容版本 Previous，
			// 让下一轮 check 重新下载（F3 逃生口）。
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("apply 拒绝：待应用版本目录缺失、为空或不完整 %s（现有文件 %d，记录 %d；回退为 applied/%s，等待重新下载）",
					pendingDir, pendingCount, versionInfo.DownloadedFileCount, versionInfo.VersionPrevious))
			clearUpdateTransientState(versionInfo)
			versionInfo.Version = versionInfo.VersionPrevious
			versionInfo.VersionStatus = config.VersionStatusApplied
			if wErr := config.WriteVersion(versionInfo); wErr != nil {
				util.AppendToLog(logDir(), "update.log", fmt.Sprintf("repair write version failed: %v", wErr))
			}
			printOutput(false, fmt.Sprintf("待应用版本目录缺失、为空或不完整: %s（已回退为 applied/%s，等待重新下载）",
				pendingDir, versionInfo.Version), nil)
			return
		}

	default:
		// 未知 version_status：不猜测、绝不替换（此前会 fall through 到 downloaded 流程）(F6)。
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("apply 拒绝：未知 version_status %q（不替换）", versionInfo.VersionStatus))
		printOutput(false, fmt.Sprintf("unknown version_status %q, refuse to apply", versionInfo.VersionStatus), nil)
		return
	}

	// Set version_status = "applying"
	versionInfo.VersionStatus = config.VersionStatusApplying
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	versionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.Version)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// 原子替换 + 重试：替换失败多因进程占用文件夹，每次重试前都会重新关闭占用进程。
	const maxAttempts = 3
	const retryInterval = 2 * time.Second

	// 共享探测状态：本次 apply 内 3 次 rename + 3 次整体重试复用同一探测/击杀结果，
	// 避免每次重试都重新全量扫描（#23）。
	st := newRenameProbeState()

	var applyErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		applyErr = applyReplacement(fc, versionInfo, versionDir, closeTimeout, st)
		if applyErr == nil {
			break
		}
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("apply attempt %d/%d failed: %v", attempt, maxAttempts, applyErr))
		if attempt < maxAttempts {
			// 回滚失败可能已导致 ApplicationFolder 丢失，此时重试只会从不存在源复制、无法恢复
			if _, statErr := os.Stat(fc.MainFolder); os.IsNotExist(statErr) {
				break
			}
			time.Sleep(retryInterval)
		}
	}
	if applyErr != nil {
		// 失败兜底：状态回退 + 启动旧版本主程序 + 附带错误信息
		printOutput(false, applyFailureFallback(fc, versionInfo, applyErr), nil)
		return
	}

	// Update version.json
	clearUpdateTransientState(versionInfo)
	versionInfo.VersionStatus = config.VersionStatusApplied
	if err := config.WriteVersion(versionInfo); err != nil {
		printOutput(false, fmt.Sprintf("write version: %v", err), nil)
		return
	}

	// F2：首次更新（Previous 为空）刚生成的备份是无版本号目录（ApplicationFolder_），
	// 不可回滚、不会被 prune 清理 → 成功且确认不再引用后删除，避免每台机器多占一份应用体积。
	cleanupOrphanBackupIfUnreferenced(fc, versionInfo)

	// Run post-update script if configured
	runPostApplyScript(fc, versionInfo.AfterApplyUpdateScript, versionInfo.Version)

	// Launch main exe
	launchMainExe(fc.ExeCfg, fc.MainFolder)

	// Output success
	printOutput(true, "", nil)
}

// applyReplacement 执行一次原子替换：按名关闭业务进程 → 复制 → 备份改名 → 替换改名。
// 任何一步失败都会尝试回滚恢复 mainFolder，并返回错误（由调用方决定是否重试）。
//
// 乐观重命名（问题 3）：不再在复制前无条件全目录扫描占用者，而是先直接重命名，
// 只在 rename 真正失败时才由 renameDirWithKillState 内部探测/击杀占用者——无占用场景
// 零扫描，最大限度节省时间；st 为本次 apply 内共享的探测状态（跨 3 次 rename 复用）。
func applyReplacement(fc *FullConfig, versionInfo *config.VersionInfo, versionDir string, closeTimeout time.Duration, st *renameProbeState) error {
	// 关闭 must_close_process_name 指定的进程（业务必需：复制前关闭主程序，
	// 否则 CopyDirWithExclude 读文件可能失败）。
	if len(fc.ExeCfg.MustCloseProcessName) > 0 {
		closeProcessesForApply(fc.ExeCfg.MustCloseProcessName)
	}
	// 不再调用 closeProcessesHoldingFolder 无条件全目录扫描——改成"先重命名，失败再查杀"，
	// 与 rollback 路径行为对齐（#2 / 问题 3）。

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
		return fmt.Errorf("copy to version dir: %v", err)
	}

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.VersionPrevious)
	if err != nil {
		return err
	}
	// 防御：Version == VersionPrevious（异常/遗留状态）时备份目录与版本目录同路径，
	// 旁移-备份-激活会自毁（静默装回旧版本，Bug#3 变体）→ 明确报错交由失败兜底处理。
	if versionInfo.VersionPrevious != "" && versionInfo.VersionPrevious == versionInfo.Version {
		return fmt.Errorf("版本状态异常：Version == VersionPrevious(%s)，拒绝替换", versionInfo.Version)
	}
	oldBackupTemp := prevVersionDir + ".old"
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		// 旧备份目录存在：必须先挪开，否则主目录重命名会因目标非空目录报 Access denied。
		// 挪不动（被占用）则直接失败，不再静默继续。
		if err := renameDirWithKillState(prevVersionDir, oldBackupTemp, closeTimeout, st); err != nil {
			return fmt.Errorf("backup aside failed: %v", err)
		}
	}

	// Rename mainFolder -> prevVersionDir (backup)
	if err := renameDirWithKillState(fc.MainFolder, prevVersionDir, closeTimeout, st); err != nil {
		// Restore old backup if it existed
		if _, statErr := os.Stat(oldBackupTemp); statErr == nil {
			if rerr := os.Rename(oldBackupTemp, prevVersionDir); rerr != nil {
				exeDir := logDir()
				util.AppendToLog(exeDir, "update.log", fmt.Sprintf("rollback restore backup: %v", rerr))
			}
		}
		return fmt.Errorf("backup rename failed: %v", err)
	}

	// Rename versionDir -> mainFolder
	// F5：改名前把"已进入激活阶段"写进 version.json。崩溃恢复据此区分
	//   (a) 改名确实发生过 → 版本目录被消耗（apply_update 的 applying + 主目录在 + 版本目录不在）；
	//   (b) 版本目录本来就不存在（状态被手工改坏 / 下载产物丢失）→ 不能凭空写 applied。
	// 写失败不阻断替换（标记只是"证据"，缺失时崩溃恢复退回历史启发式并留痕）。
	if versionInfo.VersionStatus == config.VersionStatusApplying {
		versionInfo.ApplyStage = config.ApplyStageActivating
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("write apply_stage failed: %v", wErr))
		}
	}
	if err := renameDirWithKillState(versionDir, fc.MainFolder, closeTimeout, st); err != nil {
		// Attempt rollback: rename prevVersionDir back to mainFolder
		if rerr := os.Rename(prevVersionDir, fc.MainFolder); rerr != nil {
			exeDir := logDir()
			util.AppendToLog(exeDir, "update.log", fmt.Sprintf("rollback main rename: %v", rerr))
		}
		if rerr := os.Rename(oldBackupTemp, prevVersionDir); rerr != nil {
			exeDir := logDir()
			util.AppendToLog(exeDir, "update.log", fmt.Sprintf("rollback backup restore: %v", rerr))
		}
		return fmt.Errorf("apply rename failed: %v", err)
	}

	// Clean up old backup AND any aside variants AFTER successful rename（#18）
	removeAsideVariants(prevVersionDir)
	return nil
}

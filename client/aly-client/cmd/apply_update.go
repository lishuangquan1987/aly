package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

// ApplyUpdate applies a downloaded update with atomic replacement
func ApplyUpdate() {
	fs := flag.NewFlagSet("apply_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	closeTimeoutFlag := fs.Int("close-timeout", 30, "timeout seconds for process close")
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
				versionInfo.VersionStatus = config.VersionStatusApplied
				versionInfo.RollbackPrevious = ""
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(logDir(), "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				if versionInfo.AfterApplyUpdateScript != "" {
					runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript), fc.MainFolder)
				}
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
				whitelist := buildKillWhitelist(fc)
				if err := renameDirWithKill(versionDir, fc.MainFolder, whitelist, closeTimeout); err != nil {
					printOutput(false, fmt.Sprintf("crash recovery failed: %v", err), nil)
					return
				}
				// Update status to applied
				versionInfo.VersionStatus = config.VersionStatusApplied
				versionInfo.RollbackPrevious = ""
				if wErr := config.WriteVersion(versionInfo); wErr != nil {
					util.AppendToLog(logDir(), "update.log", fmt.Sprintf("crash recovery: write version failed: %v", wErr))
				}
				// Run post-update script and launch main exe
				if versionInfo.AfterApplyUpdateScript != "" {
					runScript(filepath.Join(fc.MainFolder, versionInfo.AfterApplyUpdateScript), fc.MainFolder)
				}
				launchMainExe(fc.ExeCfg, fc.MainFolder)
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

	versionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.Version)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// 原子替换 + 重试：替换失败多因进程占用文件夹，每次重试前都会重新关闭占用进程。
	const maxAttempts = 3
	const retryInterval = 2 * time.Second

	var applyErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		applyErr = applyReplacement(fc, versionInfo, versionDir, closeTimeout)
		if applyErr == nil {
			break
		}
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("apply attempt %d/%d failed: %v", attempt, maxAttempts, applyErr))
		// 非白名单进程占用（请关闭后重试）：重试无意义，直接失败兜底
		if strings.Contains(applyErr.Error(), "请关闭后重试") {
			break
		}
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

	// Output success
	printOutput(true, "", nil)
}

// applyReplacement 执行一次原子替换：关闭占用进程 → 复制 → 备份改名 → 替换改名。
// 任何一步失败都会尝试回滚恢复 mainFolder，并返回错误（由调用方决定是否重试）。
func applyReplacement(fc *FullConfig, versionInfo *config.VersionInfo, versionDir string, closeTimeout time.Duration) error {
	whitelist := buildKillWhitelist(fc)
	// 关闭 must_close_process_name 指定的进程
	if len(fc.ExeCfg.MustCloseProcessName) > 0 {
		closeProcessesGracefully(fc.ExeCfg.MustCloseProcessName, closeTimeout)
	}
	// 自动探测并结束占用 ApplicationFolder 的进程（排除更新器自身）。
	// 仅强杀白名单进程（must_close_process_name / 主程序 / explorer）；
	// 非白名单占用者返回错误，由调用方提示用户手动关闭（#13）。
	if err := closeProcessesHoldingFolder(fc.MainFolder, whitelist, closeTimeout); err != nil {
		return err
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
		return fmt.Errorf("copy to version dir: %v", err)
	}

	// Compute paths for atomic rename
	prevVersionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.VersionPrevious)
	if err != nil {
		return err
	}
	// Temporarily move old backup aside instead of deleting upfront (safer for power failure)
	oldBackupTemp := prevVersionDir + ".old"
	if err := os.RemoveAll(oldBackupTemp); err != nil {
		exeDir := logDir()
		util.AppendToLog(exeDir, "update.log", fmt.Sprintf("remove old backup temp: %v", err))
	}
	if _, statErr := os.Stat(prevVersionDir); statErr == nil {
		// 旧备份目录存在：必须先挪开，否则主目录重命名会因目标非空目录报 Access denied。
		// 挪不动（被占用）则直接失败，不再静默继续。
		if err := renameDirWithKill(prevVersionDir, oldBackupTemp, whitelist, closeTimeout); err != nil {
			return fmt.Errorf("backup aside failed: %v", err)
		}
	}

	// Rename mainFolder -> prevVersionDir (backup)
	if err := renameDirWithKill(fc.MainFolder, prevVersionDir, whitelist, closeTimeout); err != nil {
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
	if err := renameDirWithKill(versionDir, fc.MainFolder, whitelist, closeTimeout); err != nil {
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

	// Clean up old backup AFTER successful rename
	if err := os.RemoveAll(oldBackupTemp); err != nil {
		exeDir := logDir()
		util.AppendToLog(exeDir, "update.log", fmt.Sprintf("cleanup old backup: %v", err))
	}
	return nil
}

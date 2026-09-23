package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiclient "aly/client/aly-client/client"
	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

const largeFileThreshold = 100 * 1024 * 1024 // 100MB

// DownloadUpdate downloads only changed files from server
func DownloadUpdate() {
	fs := flag.NewFlagSet("download_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

	// 全局更新锁：同一时刻只允许一个更新操作（下载/应用/回滚）
	releaseLock, lockErr := AcquireUpdateLock("download_update")
	if lockErr != nil {
		printOutput(false, lockErr.Error(), nil)
		return
	}
	defer releaseLock()

	fc, err := loadFullConfig(*urlFlag, *projectNameFlag, *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	if fc.Shared.ServerURL == "" {
		printOutput(false, "no server url configured", nil)
		return
	}
	if fc.Shared.ProjectName == "" {
		printOutput(false, "no project name configured", nil)
		return
	}

	logs, err := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	if len(logs) == 0 {
		printOutput(false, "no change logs on server", nil)
		return
	}

	latestLog := findLatestLog(logs)
	newVersion := stripVPrefix(latestLog.Version)

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}
	currentVersion := stripVPrefix(versionInfo.Version)

	// applying 期间：不重复下载、不改写版本状态。由 apply_update 走崩溃恢复分支完成在途操作
	// （修复 Bug#3：download 不得把 applying 降级成 downloaded、不得覆盖 VersionPrevious/RollbackPrevious）。
	if versionInfo.VersionStatus == config.VersionStatusApplying {
		util.AppendToLog(logDir(), "download.log", "applying 进行中，跳过 download（由 apply_update 恢复）")
		printOutput(true, "", &model.DownloadUpdateData{Version: currentVersion})
		return
	}

	// Guard: if this exact version was already downloaded, skip re-download.
	if versionInfo.VersionStatus == config.VersionStatusDownloaded &&
		currentVersion == newVersion {
		printOutput(true, "", &model.DownloadUpdateData{Version: newVersion})
		return
	}
	if compareVersion(newVersion, currentVersion) <= 0 {
		printOutput(false, "already at latest version", nil)
		return
	}

	serverFiles, err := apiclient.GetAllFiles(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, fmt.Sprintf("get file list: %v", err), nil)
		return
	}

	localMD5Map, localMD5Err := util.LocalFileMD5Map(fc.MainFolder)
	if localMD5Err != nil {
		exeDir, _ := config.ExeDir()
		util.AppendToLog(exeDir, "download.log", fmt.Sprintf("local md5 scan warning: %v", localMD5Err))
	}

	targetDir, err := fc.ExeCfg.AppVersionDir(newVersion)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	if err := util.EnsureDir(targetDir); err != nil {
		printOutput(false, fmt.Sprintf("create target dir: %v", err), nil)
		return
	}

	// 只打印真正需要下载的差异文件：本地已匹配、或目标目录已有正确文件的
	// 都不输出（不打印 SKIP），total 取差异文件数，避免界面按行统计进度出错。
	// 逻辑：
	// - 本地当前版本 MD5 已匹配 → 无需下载（不打印）
	// - 目标版本目录已有正确文件（断点续传/已下载过）→ 无需下载（不打印）
	// - 否则 → START → 下载（3 次重试 + MD5/SHA256 校验）→ DONE
	type fileToDownload struct {
		idx        int
		serverFile model.FileInfo
	}
	var downloadList []fileToDownload
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)

		// 本地当前版本已匹配：非差异文件，无需下载
		localMD5, localExists := localMD5Map[relPath]
		if localExists && localMD5 == serverFiles[i].MD5 {
			continue
		}

		// 目标目录已有正确文件（MD5+SHA256）：无需重新下载
		localPath := filepath.Join(targetDir, filepathFromSlash(relPath))
		if info, statErr := os.Stat(localPath); statErr == nil && info.Size() == serverFiles[i].FileSize {
			targetMD5, md5Err := util.FileMD5(localPath)
			targetSHA256, shaErr := util.FileSHA256(localPath)
			if md5Err == nil && shaErr == nil && targetMD5 == serverFiles[i].MD5 && targetSHA256 == serverFiles[i].SHA256 {
				continue
			}
		}

		downloadList = append(downloadList, fileToDownload{idx: len(downloadList) + 1, serverFile: serverFiles[i]})
	}

	total := len(downloadList)
	for _, dl := range downloadList {
		relPath := normalizePath(dl.serverFile.FileRelativePath)
		localPath := filepath.Join(targetDir, filepathFromSlash(relPath))

		printProgress(dl.idx, total, relPath, "START", dl.serverFile.FileSize, "")

		// Download with retry up to 3 times
		var lastErr string
		for retry := 0; retry < 3; retry++ {
			if err := apiclient.DownloadFileWithResume(fc.Shared.ServerURL, dl.serverFile.FileAbsolutePath, localPath, dl.serverFile.FileSize, largeFileThreshold); err != nil {
				lastErr = fmt.Sprintf("download error: %v", err)
				if retry == 2 {
					exeDir, _ := config.ExeDir()
					util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
						fmt.Sprintf("%s %s", dl.serverFile.FileRelativePath, lastErr))
					printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
					return
				}
				continue
			}

			// Verify MD5 + SHA256
			localMD5, md5Err := util.FileMD5(localPath)
			localSHA256, shaErr := util.FileSHA256(localPath)

			if md5Err != nil || shaErr != nil {
				lastErr = "hash compute error"
				if retry == 2 {
					exeDir, _ := config.ExeDir()
					util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
						fmt.Sprintf("%s %s", dl.serverFile.FileRelativePath, lastErr))
					printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
					return
				}
				os.Remove(localPath)
				continue
			}

			if localMD5 == dl.serverFile.MD5 && localSHA256 == dl.serverFile.SHA256 {
				break
			}

			lastErr = "checksum mismatch"
			if retry == 2 {
				exeDir, _ := config.ExeDir()
				util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
					fmt.Sprintf("%s %s (server_md5=%s local_md5=%s)", dl.serverFile.FileRelativePath, lastErr, dl.serverFile.MD5, localMD5))
				printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
				return
			}
			os.Remove(localPath)
		}

		printProgress(dl.idx, total, relPath, "DONE", dl.serverFile.FileSize, "")
	}

	// Update version.json
	// VersionPrevious 语义 = "MainFolder 当前真实内容版本"，仅在可确定时更新（修复 Bug#2）：
	//   - downloaded：MainFolder 仍是 VersionPrevious 的内容 → 保持不变（"待应用期间发布新版"
	//     不再把 VersionPrevious 错写成从未应用过的中间版本）
	//   - applied / 空：MainFolder = Version → 记录为 VersionPrevious
	//   （applying 已在函数开头 return，不会走到这里）
	if versionInfo.VersionStatus != config.VersionStatusDownloaded {
		versionInfo.VersionPrevious = versionInfo.Version
	}
	versionInfo.Version = newVersion
	versionInfo.VersionStatus = config.VersionStatusDownloaded
	versionInfo.RollbackPrevious = ""
	versionInfo.AfterApplyUpdateScript = latestLog.AfterApplyUpdateScript
	if err := config.WriteVersion(versionInfo); err != nil {
		printProgressFail(0, 0, "version.json", 0, fmt.Sprintf("write version: %v", err))
		return
	}

	printProgressDone()
}

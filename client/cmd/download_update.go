package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiclient "zap/client/client"
	"zap/client/config"
	"zap/client/model"
	"zap/client/util"
)

const largeFileThreshold = 100 * 1024 * 1024 // 100MB

// DownloadUpdate downloads only changed files from server
func DownloadUpdate() {
	fs := flag.NewFlagSet("download_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

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

	latestLog := logs[0]
	for i := 1; i < len(logs); i++ {
		if logs[i].ID > latestLog.ID {
			latestLog = logs[i]
		}
	}
	newVersion := stripVPrefix(latestLog.Version)

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}
	currentVersion := stripVPrefix(versionInfo.Version)

	// Guard: if this exact version was already downloaded, skip re-download.
	// Exception: when status is "applying" (crash recovery), allow re-download
	// even if same version, since downloaded files may be corrupted.
	if versionInfo.VersionStatus != config.VersionStatusApplying {
		if versionInfo.VersionStatus == config.VersionStatusDownloaded &&
			currentVersion == newVersion {
			printOutput(true, "", &model.DownloadUpdateData{Version: newVersion})
			return
		}
		if compareVersion(newVersion, currentVersion) <= 0 {
			printOutput(false, "already at latest version", nil)
			return
		}
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

	// 遍历服务端所有文件，输出完整的进度汇报到 stdout。
	// 逻辑：单循环遍历所有 server 文件，统一计数器 progIdx（1-based），
	// 确保 SKIP / START / DONE 序号连续不重复。
	// - 本地 MD5 已匹配 → SKIP（文件未变化，无需下载）
	// - 本地不存在或不匹配 → 检查目标目录，已有正确文件 → SKIP
	// - 否则 → START → 下载（3 次重试 + MD5/SHA256 校验）→ DONE
	total := len(serverFiles)
	type fileToDownload struct {
		idx        int
		serverFile model.FileInfo
	}
	var downloadList []fileToDownload
	progIdx := 0
	for i := range serverFiles {
		progIdx++
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localMD5, localExists := localMD5Map[relPath]
		if localExists && localMD5 == serverFiles[i].MD5 {
			printProgress(progIdx, total, relPath, "SKIP", serverFiles[i].FileSize, "")
			continue
		}
		downloadList = append(downloadList, fileToDownload{progIdx, serverFiles[i]})
	}

	for _, dl := range downloadList {
		relPath := normalizePath(dl.serverFile.FileRelativePath)
		localPath := filepath.Join(targetDir, filepathFromSlash(relPath))

		// 目标目录已有正确文件（MD5+SHA256），跳过下载
		if info, statErr := os.Stat(localPath); statErr == nil && info.Size() == dl.serverFile.FileSize {
			localMD5, md5Err := util.FileMD5(localPath)
			localSHA256, shaErr := util.FileSHA256(localPath)
			if md5Err == nil && shaErr == nil && localMD5 == dl.serverFile.MD5 && localSHA256 == dl.serverFile.SHA256 {
				printProgress(dl.idx, total, relPath, "SKIP", dl.serverFile.FileSize, "")
				continue
			}
		}

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
	versionInfo.VersionPrevious = versionInfo.Version
	versionInfo.Version = newVersion
	versionInfo.VersionStatus = config.VersionStatusDownloaded
	versionInfo.AfterApplyUpdateScript = latestLog.AfterApplyUpdateScript
	if err := config.WriteVersion(versionInfo); err != nil {
		printProgressFail(0, 0, "version.json", 0, fmt.Sprintf("write version: %v", err))
		return
	}

	printProgressDone()
}

package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiclient "clientupdator/client/client"
	"clientupdator/client/config"
	"clientupdator/client/model"
	"clientupdator/client/util"
)

const largeFileThreshold = 100 * 1024 * 1024 // 100MB

// DownloadUpdate downloads only changed files from server
func DownloadUpdate() {
	fs := flag.NewFlagSet("download_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig(*urlFlag, *projectNameFlag, *mainExePathFlag, "")
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

	// Guard: skip version.json update if this exact version was already downloaded
	skipVersionUpdate := versionInfo.VersionStatus == config.VersionStatusDownloaded &&
		currentVersion == newVersion

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

	var diffFiles []model.FileInfo
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localMD5, localExists := localMD5Map[relPath]
		if !localExists || localMD5 != serverFiles[i].MD5 {
			diffFiles = append(diffFiles, serverFiles[i])
		}
	}

	for _, serverFile := range diffFiles {
		localPath := filepath.Join(targetDir, filepathFromSlash(normalizePath(serverFile.FileRelativePath)))

		// Check if file already exists in target dir with correct MD5+SHA256, skip if valid
		if info, statErr := os.Stat(localPath); statErr == nil && info.Size() == serverFile.FileSize {
			localMD5, md5Err := util.FileMD5(localPath)
			localSHA256, shaErr := util.FileSHA256(localPath)
			if md5Err == nil && shaErr == nil && localMD5 == serverFile.MD5 && localSHA256 == serverFile.SHA256 {
				continue
			}
		}

		// Download with retry up to 3 times
		downloaded := false
		var lastErr string
		for retry := 0; retry < 3; retry++ {
			if err := apiclient.DownloadFileWithResume(fc.Shared.ServerURL, serverFile.FileAbsolutePath, localPath, serverFile.FileSize, largeFileThreshold); err != nil {
				lastErr = fmt.Sprintf("download error: %v", err)
				if retry == 2 {
					exeDir, _ := config.ExeDir()
					util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
						fmt.Sprintf("%s %s", serverFile.FileRelativePath, lastErr))
					printOutput(false, fmt.Sprintf("download %s failed after 3 retries: %v", serverFile.FileRelativePath, err), nil)
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
						fmt.Sprintf("%s %s", serverFile.FileRelativePath, lastErr))
					printOutput(false, fmt.Sprintf("hash compute %s failed after 3 retries", serverFile.FileRelativePath), nil)
					return
				}
				os.Remove(localPath)
				continue
			}

			if localMD5 == serverFile.MD5 && localSHA256 == serverFile.SHA256 {
				downloaded = true
				break
			}

			lastErr = "checksum mismatch"
			if retry == 2 {
				exeDir, _ := config.ExeDir()
				util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
					fmt.Sprintf("%s %s (server_md5=%s local_md5=%s)", serverFile.FileRelativePath, lastErr, serverFile.MD5, localMD5))
				printOutput(false, fmt.Sprintf("verify %s failed after 3 retries", serverFile.FileRelativePath), nil)
				return
			}
			os.Remove(localPath)
		}

		if !downloaded {
			return
		}
	}

	// Update version.json ONLY if status != "downloaded"
	if !skipVersionUpdate {
		versionInfo.VersionPrevious = versionInfo.Version
		versionInfo.Version = newVersion
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if err := config.WriteVersion(versionInfo); err != nil {
			printOutput(false, fmt.Sprintf("write version: %v", err), nil)
			return
		}
	}

	printOutput(true, "", model.DownloadUpdateData{Version: newVersion})
}

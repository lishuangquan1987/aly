package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiclient "clientupdator/client/http_client"
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
	silentFlag := fs.Bool("silent", false, "silent mode")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("load config: %v", err)})
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, *mainExePathFlag, "")

	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: err.Error()})
		return
	}

	logs, _ := apiclient.GetProjectChangeLogs(cfg.URL, project.ID)
	if len(logs) == 0 {
		printJSON(model.SuccessOutput{Success: false, Error: "no change logs on server"})
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
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("read version: %v", err)})
		return
	}
	currentVersion := stripVPrefix(versionInfo.Version)

	if compareVersion(newVersion, currentVersion) <= 0 {
		printJSON(model.SuccessOutput{Success: false, Error: "already at latest version"})
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: err.Error()})
		return
	}

	serverFiles, err := apiclient.GetAllFiles(cfg.URL, project.ID)
	if err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("get file list: %v", err)})
		return
	}

	localMD5Map, _ := util.LocalFileMD5Map(mainFolder)

	updateDir := filepath.Join(mainFolder, "update", newVersion)
	util.EnsureDir(updateDir)

	var diffFiles []model.FileInfo
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localMD5, localExists := localMD5Map[relPath]
		if !localExists || localMD5 != serverFiles[i].MD5 {
			diffFiles = append(diffFiles, serverFiles[i])
		}
	}

	totalFiles := len(diffFiles)
	if !*silentFlag {
		fmt.Fprintf(os.Stderr, "progress:%d files to download\n", totalFiles)
	}

	failLogPath := filepath.Join(updateDir, fmt.Sprintf("update_%s_fail.log", newVersion))
	var failedFiles []string

	for idx, serverFile := range diffFiles {
		relPath := normalizePath(serverFile.FileRelativePath)
		localPath := filepath.Join(updateDir, filepath.FromSlash(relPath))

		if !*silentFlag {
			fmt.Fprintf(os.Stderr, "progress:%d/%d files\n", idx+1, totalFiles)
			fmt.Fprintf(os.Stderr, "progress:downloading %s\n", relPath)
		}

		// 下载并校验，最多重试3次
		var downloadSuccess bool
		for retry := 0; retry < 3; retry++ {
			if retry > 0 && !*silentFlag {
				fmt.Fprintf(os.Stderr, "progress:retrying %s (%d/3)\n", relPath, retry+1)
			}

			// 清理可能存在的旧文件
			os.Remove(localPath)
			os.Remove(localPath + ".part")

			if err := apiclient.DownloadFileWithResume(cfg.URL, serverFile.FileAbsolutePath, localPath, serverFile.FileSize, largeFileThreshold); err != nil {
				if retry == 2 {
					failedFiles = append(failedFiles, fmt.Sprintf("download %s failed: %v", relPath, err))
				}
				continue
			}

			// 校验 MD5 和 SHA256
			localMD5, md5Err := util.FileMD5(localPath)
			localSHA256, shaErr := util.FileSHA256(localPath)

			if md5Err != nil || shaErr != nil {
				if retry == 2 {
					failedFiles = append(failedFiles, fmt.Sprintf("hash compute %s failed: md5=%v, sha256=%v", relPath, md5Err, shaErr))
				}
				continue
			}

			if localMD5 != serverFile.MD5 || localSHA256 != serverFile.SHA256 {
				if retry == 2 {
					failedFiles = append(failedFiles, fmt.Sprintf("verify %s failed: md5=%s vs %s, sha256=%s vs %s", relPath, localMD5, serverFile.MD5, localSHA256, serverFile.SHA256))
				}
				continue
			}

			if !*silentFlag {
				fmt.Fprintf(os.Stderr, "progress:verifying %s sha256 ok\n", relPath)
			}
			downloadSuccess = true
			break
		}

		if !downloadSuccess {
			continue
		}
	}

	if len(failedFiles) > 0 {
		// 写入失败日志
		f, _ := os.Create(failLogPath)
		if f != nil {
			for _, msg := range failedFiles {
				f.WriteString(msg + "\n")
			}
			f.Close()
		}
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("%d files failed", len(failedFiles))})
		return
	}

	// 5. 将 main_exe_relative_path_folder 中的所有文件复制到 update/{new_version}/，已存在的文件不覆盖
	if !*silentFlag {
		fmt.Fprintf(os.Stderr, "progress:copying local files to update dir\n")
	}
	if err := util.CopyDir(mainFolder, updateDir, false); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("copy local files failed: %v", err)})
		return
	}

	// 6. 将 main_exe_relative_path_folder 中所有文件备份到 update/{current_version}/
	currentVersionDir := filepath.Join(mainFolder, "update", currentVersion)
	if !*silentFlag {
		fmt.Fprintf(os.Stderr, "progress:backing up current version\n")
	}
	if err := util.CopyDir(mainFolder, currentVersionDir, true); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("backup failed: %v", err)})
		return
	}

	// 7. 更新 version.json
	versionInfo.Version = newVersion
	versionInfo.VersionStatus = config.VersionStatusDownloaded
	if err := config.WriteVersion(versionInfo); err != nil {
		printJSON(model.SuccessOutput{Success: false, Error: fmt.Sprintf("write version: %v", err)})
		return
	}

	if !*silentFlag {
		fmt.Fprintf(os.Stderr, "progress:%d/%d files complete\n", totalFiles, totalFiles)
	}
	printJSON(model.SuccessOutput{Success: true, Version: newVersion})
}

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

// CheckDiff lists files that differ from server (only server-side files)
func CheckDiff() {
	fs := flag.NewFlagSet("check_diff", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		printOutput(false, fmt.Sprintf("load config: %v", err), nil)
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, "", "")

	if cfg.URL == "" {
		printOutput(false, "no server url configured", nil)
		return
	}
	if cfg.ProjectName == "" {
		printOutput(false, "no project name configured", nil)
		return
	}

	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	serverFiles, err := apiclient.GetAllFiles(cfg.URL, project.ID)
	if err != nil {
		printOutput(false, fmt.Sprintf("get server files: %v", err), nil)
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	// Get new version from change logs
	logs, logsErr := apiclient.GetProjectChangeLogs(cfg.URL, project.ID)
	newVersion := ""
	if logsErr == nil && len(logs) > 0 {
		latestLog := logs[0]
		for i := 1; i < len(logs); i++ {
			if logs[i].ID > latestLog.ID {
				latestLog = logs[i]
			}
		}
		newVersion = stripVPrefix(latestLog.Version)
	}

	// Build local file info maps
	localMD5Map, localMD5Err := util.LocalFileMD5Map(mainFolder)
	if localMD5Err != nil {
		exeDir, _ := config.ExeDir()
		util.AppendToLog(exeDir, "check_diff.log", fmt.Sprintf("local md5 scan warning: %v", localMD5Err))
	}

	var diffFiles []model.DiffFileItem
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localFullPath := filepath.Join(mainFolder, filepathFromSlash(relPath))
		localMD5, localExists := localMD5Map[relPath]

		if !localExists {
			diffFiles = append(diffFiles, model.DiffFileItem{
				Path:         relPath,
				LocalMD5:     "",
				LocalSize:    0,
				LocalSHA256:  "",
				ServerMD5:    serverFiles[i].MD5,
				ServerSize:   serverFiles[i].FileSize,
				ServerSHA256: serverFiles[i].SHA256,
			})
		} else if localMD5 != serverFiles[i].MD5 {
			localSize := int64(0)
			if info, err := os.Stat(localFullPath); err == nil {
				localSize = info.Size()
			}
			localSHA256, _ := util.FileSHA256(localFullPath)
			diffFiles = append(diffFiles, model.DiffFileItem{
				Path:         relPath,
				LocalMD5:     localMD5,
				LocalSize:    localSize,
				LocalSHA256:  localSHA256,
				ServerMD5:    serverFiles[i].MD5,
				ServerSize:   serverFiles[i].FileSize,
				ServerSHA256: serverFiles[i].SHA256,
			})
		}
	}

	printOutput(true, "", &model.CheckDiffData{
		NewVersion: newVersion,
		Files:      diffFiles,
	})
}




package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	apiclient "zap/client/client"
	"zap/client/config"
	"zap/client/model"
	"zap/client/util"
)

// CheckDiff lists files that differ from server (only server-side files)
func CheckDiff() {
	fs := flag.NewFlagSet("check_diff", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig(*urlFlag, *projectNameFlag, "")
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

	serverFiles, err := apiclient.GetAllFiles(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, fmt.Sprintf("get server files: %v", err), nil)
		return
	}

	// Get new version from change logs
	logs, logsErr := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
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
	localMD5Map, localMD5Err := util.LocalFileMD5Map(fc.MainFolder)
	if localMD5Err != nil {
		exeDir, _ := config.ExeDir()
		util.AppendToLog(exeDir, "check_diff.log", fmt.Sprintf("local md5 scan warning: %v", localMD5Err))
	}

	var diffFiles []model.DiffFileItem
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localFullPath := filepath.Join(fc.MainFolder, filepathFromSlash(relPath))
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
			diffFiles = append(diffFiles, model.DiffFileItem{
				Path:         relPath,
				LocalMD5:     localMD5,
				LocalSize:    localSize,
				LocalSHA256:  "", // filled in parallel below
				ServerMD5:    serverFiles[i].MD5,
				ServerSize:   serverFiles[i].FileSize,
				ServerSHA256: serverFiles[i].SHA256,
			})
		}
	}

	// Compute SHA256 in parallel for files that differ by MD5
	var shaWg sync.WaitGroup
	for i := range diffFiles {
		if diffFiles[i].LocalMD5 == "" {
			continue // file doesn't exist locally, no SHA256 needed
		}
		shaWg.Add(1)
		go func(idx int) {
			defer shaWg.Done()
			localFullPath := filepath.Join(fc.MainFolder, filepathFromSlash(diffFiles[idx].Path))
			sha256, _ := util.FileSHA256(localFullPath)
			diffFiles[idx].LocalSHA256 = sha256
		}(i)
	}
	shaWg.Wait()

	printOutput(true, "", &model.CheckDiffData{
		NewVersion: newVersion,
		Files:      diffFiles,
	})
}

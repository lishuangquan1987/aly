package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "clientupdator/client/http_client"
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
		fmt.Fprintf(os.Stderr, "load config error: %v\n", err)
		printJSON(model.CheckDiffOutput{})
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, "", "")

	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		printJSON(model.CheckDiffOutput{})
		return
	}

	serverFiles, err := apiclient.GetAllFiles(cfg.URL, project.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "get server files error: %v\n", err)
		printJSON(model.CheckDiffOutput{})
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "main folder error: %v\n", err)
		printJSON(model.CheckDiffOutput{})
		return
	}

	localMD5Map, err := util.LocalFileMD5Map(mainFolder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "local md5 error: %v\n", err)
		printJSON(model.CheckDiffOutput{})
		return
	}

	// Get new version
	logs, _ := apiclient.GetProjectChangeLogs(cfg.URL, project.ID)
	newVersion := ""
	if len(logs) > 0 {
		latestLog := logs[0]
		for i := 1; i < len(logs); i++ {
			if logs[i].ID > latestLog.ID {
				latestLog = logs[i]
			}
		}
		newVersion = stripVPrefix(latestLog.Version)
	}

	var diffFiles []model.DiffFile
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localMD5, localExists := localMD5Map[relPath]
		if !localExists {
			diffFiles = append(diffFiles, model.DiffFile{
				Path:       relPath,
				Status:     "new",
				LocalMD5:   "",
				ServerMD5:  serverFiles[i].MD5,
				ServerSize: serverFiles[i].FileSize,
			})
		} else if localMD5 != serverFiles[i].MD5 {
			diffFiles = append(diffFiles, model.DiffFile{
				Path:       relPath,
				Status:     "modified",
				LocalMD5:   localMD5,
				ServerMD5:  serverFiles[i].MD5,
				ServerSize: serverFiles[i].FileSize,
			})
		}
	}

	printJSON(model.CheckDiffOutput{
		NewVersion: newVersion,
		Files:      diffFiles,
	})
}

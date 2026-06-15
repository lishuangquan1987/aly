package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "zap/client/client"
	"zap/client/config"
	"zap/client/model"
)

// CheckUpdate checks for new version
func CheckUpdate() {
	fs := flag.NewFlagSet("check_update", flag.ExitOnError)
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

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}

	localVersion := stripVPrefix(versionInfo.Version)
	status := versionInfo.VersionStatus

	// 尝试连接服务器获取项目信息和变更日志
	project, projectErr := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
	logs, logsErr := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)

	serverReachable := projectErr == nil && logsErr == nil && len(logs) > 0

	var serverVersion string
	var forceUpdate *bool
	if serverReachable {
		latestLog := logs[0]
		for i := 1; i < len(logs); i++ {
			if logs[i].ID > latestLog.ID {
				latestLog = logs[i]
			}
		}
		serverVersion = stripVPrefix(latestLog.Version)
		forceUpdate = &project.ForceUpdate
	}

	versionsMatch := serverReachable && serverVersion == localVersion

	switch {
	case status == "" || status == config.VersionStatusApplied:
		// 已应用或无版本文件：只有服务器有更新版本时才需要下载
		if versionsMatch || !serverReachable {
			printOutput(true, "", &model.CheckUpdateData{
				HasUpdate:      false,
				CurrentVersion: localVersion,
			})
		} else {
			printOutput(true, "", &model.CheckUpdateData{
				HasUpdate:          true,
				NeedDownloadUpdate: true,
				CurrentVersion:     localVersion,
				NewVersion:         serverVersion,
				ForceUpdate:        forceUpdate,
			})
		}

	case status == config.VersionStatusDownloaded || status == config.VersionStatusApplying:
		// 已下载或正在应用：优先继续 apply，只有服务器版本不一致才重新下载
		if versionsMatch || !serverReachable {
			currentVer := versionInfo.VersionPrevious
			if currentVer == "" {
				currentVer = versionInfo.Version
			}
			printOutput(true, "", &model.CheckUpdateData{
				HasUpdate:          true,
				NeedDownloadUpdate: false,
				CurrentVersion:     stripVPrefix(currentVer),
				NewVersion:         stripVPrefix(versionInfo.Version),
			})
		} else {
			printOutput(true, "", &model.CheckUpdateData{
				HasUpdate:          true,
				NeedDownloadUpdate: true,
				CurrentVersion:     localVersion,
				NewVersion:         serverVersion,
				ForceUpdate:        forceUpdate,
			})
		}
	}
}

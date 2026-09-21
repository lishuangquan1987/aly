package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "aly/client/aly-client/client"
	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
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

	switch {
	case status == "" || status == config.VersionStatusApplied:
		checkUpdateApplied(fc, localVersion)

	case status == config.VersionStatusDownloaded || status == config.VersionStatusApplying:
		checkUpdatePending(fc, versionInfo, localVersion)

	default:
		// 未知状态视为 applied，避免静默失败
		fmt.Fprintf(os.Stderr, "check_update: unknown version_status %q, treating as applied\n", status)
		checkUpdateApplied(fc, localVersion)
	}
}

// checkUpdateApplied 处理 applied / 空状态：必须联系服务器判断是否有更新
func checkUpdateApplied(fc *FullConfig, localVersion string) {
	project, projectErr := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if projectErr != nil {
		fmt.Fprintf(os.Stderr, "check_update: FindProjectByName failed: %v\n", projectErr)
	}
	logs, logsErr := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if logsErr != nil {
		fmt.Fprintf(os.Stderr, "check_update: GetProjectChangeLogs failed: %v\n", logsErr)
	}

	serverReachable := projectErr == nil && logsErr == nil && len(logs) > 0
	if !serverReachable {
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      false,
			CurrentVersion: localVersion,
		})
		return
	}

	latestLog := findLatestLog(logs)
	serverVersion := stripVPrefix(latestLog.Version)

	if !needUpdate(serverVersion, localVersion) {
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      false,
			CurrentVersion: localVersion,
		})
	} else {
		forceUpdate := project.ForceUpdate
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: true,
			CurrentVersion:     localVersion,
			NewVersion:         serverVersion,
			ForceUpdate:        &forceUpdate,
		})
	}
}

// checkUpdatePending 处理 downloaded / applying 状态：优先继续 apply，只有服务器版本不一致才重新下载
func checkUpdatePending(fc *FullConfig, versionInfo *config.VersionInfo, localVersion string) {
	// 只请求变更日志（1 次 HTTP），减少断网时的等待
	logs, logsErr := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if logsErr != nil {
		fmt.Fprintf(os.Stderr, "check_update: GetProjectChangeLogs failed: %v\n", logsErr)
	}

	serverReachable := logsErr == nil && len(logs) > 0

	if !serverReachable {
		// 断网：继续 apply 已下载的版本
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
		return
	}

	latestLog := findLatestLog(logs)
	serverVersion := stripVPrefix(latestLog.Version)

	if !needUpdate(serverVersion, localVersion) {
		// 服务器版本不高于本地待应用版本：继续 apply 已下载版本
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
		// 服务器有新版本：需要重新下载，补拿 ForceUpdate
		forceUpdate := false
		project, projectErr := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
		if projectErr != nil {
			fmt.Fprintf(os.Stderr, "check_update: FindProjectByName failed: %v\n", projectErr)
		} else {
			forceUpdate = project.ForceUpdate
		}
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: true,
			CurrentVersion:     localVersion,
			NewVersion:         serverVersion,
			ForceUpdate:        &forceUpdate,
		})
	}
}

// needUpdate 统一版本判断口径：仅当服务器版本严格高于本地版本时才需要更新。
// 与 download_update 的 compareVersion(newVersion, currentVersion) > 0 语义保持一致，
// 避免服务端版本低于本地（回滚发布/连错环境）时 check 报有更新而 download 报已是最新，
// 导致开启强制更新的客户端永久卡死。
func needUpdate(serverVersion, localVersion string) bool {
	return compareVersion(serverVersion, localVersion) > 0
}

// findLatestLog 返回 ID 最大的变更日志
func findLatestLog(logs []model.ProjectChangeLog) model.ProjectChangeLog {
	latest := logs[0]
	for i := 1; i < len(logs); i++ {
		if logs[i].ID > latest.ID {
			latest = logs[i]
		}
	}
	return latest
}

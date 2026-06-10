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

	fc, err := loadFullConfig(*urlFlag, *projectNameFlag, "", "")
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

	// Read version.json. Handle downloaded/applying status first.
	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}

	if versionInfo.VersionStatus == config.VersionStatusDownloaded ||
		versionInfo.VersionStatus == config.VersionStatusApplying {
		currentVer := versionInfo.VersionPrevious
		if currentVer == "" {
			currentVer = versionInfo.Version
		}
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      true,
			CurrentVersion: stripVPrefix(currentVer),
			NewVersion:     stripVPrefix(versionInfo.Version),
		})
		return
	}

	project, err := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	logs, err := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	if len(logs) == 0 {
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      false,
			CurrentVersion: stripVPrefix(versionInfo.Version),
		})
		return
	}

	// Find latest log (highest ID)
	latestLog := logs[0]
	for i := 1; i < len(logs); i++ {
		if logs[i].ID > latestLog.ID {
			latestLog = logs[i]
		}
	}

	serverVersion := stripVPrefix(latestLog.Version)
	localVersion := stripVPrefix(versionInfo.Version)

	if compareVersion(serverVersion, localVersion) > 0 {
		forceUpdate := project.ForceUpdate
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      true,
			CurrentVersion: localVersion,
			NewVersion:     serverVersion,
			ForceUpdate:    &forceUpdate,
		})
	} else {
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      false,
			CurrentVersion: localVersion,
		})
	}
}

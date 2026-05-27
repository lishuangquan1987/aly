package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "clientupdator/client/http_client"
	"clientupdator/client/config"
	"clientupdator/client/model"
)

// CheckUpdate checks for new version
func CheckUpdate() {
	fs := flag.NewFlagSet("check_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, Error: fmt.Sprintf("load config: %v", err)})
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, "", "")

	if cfg.URL == "" {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, Error: "no server url configured"})
		return
	}
	if cfg.ProjectName == "" {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, Error: "no project name configured"})
		return
	}

	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, Error: err.Error()})
		return
	}

	logs, err := apiclient.GetProjectChangeLogs(cfg.URL, project.ID)
	if err != nil {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, Error: err.Error()})
		return
	}
	if len(logs) == 0 {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, CurrentVersion: ""})
		return
	}

	latestLog := logs[0]
	for i := 1; i < len(logs); i++ {
		if logs[i].ID > latestLog.ID {
			latestLog = logs[i]
		}
	}

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printJSON(model.CheckUpdateOutput{HasUpdate: false, Error: fmt.Sprintf("read version: %v", err)})
		return
	}

	serverVersion := stripVPrefix(latestLog.Version)
	localVersion := stripVPrefix(versionInfo.Version)

	if compareVersion(serverVersion, localVersion) > 0 {
		forceUpdate := false
		projects, perr := apiclient.GetAllProjects(cfg.URL)
		if perr == nil {
			for _, p := range projects {
				if p.ID == project.ID {
					forceUpdate = p.ForceUpdate
					break
				}
			}
		}
		printJSON(model.CheckUpdateOutput{
			HasUpdate:      true,
			CurrentVersion: localVersion,
			NewVersion:     serverVersion,
			ForceUpdate:    forceUpdate,
		})
	} else {
		printJSON(model.CheckUpdateOutput{
			HasUpdate:      false,
			CurrentVersion: localVersion,
		})
	}
}

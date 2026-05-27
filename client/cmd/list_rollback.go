package cmd

import (
	"flag"
	"os"
	"path/filepath"

	"clientupdator/client/config"
	"clientupdator/client/model"
)

// ListRollbackVersions 列出可回滚版本
func ListRollbackVersions() {
	fs := flag.NewFlagSet("list_rollback_versions", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		printJSON(model.RollbackListOutput{})
		return
	}
	cfg.MergeFlags("", "", *mainExePathFlag, "")

	updateDir, err := cfg.UpdateDir()
	if err != nil {
		printJSON(model.RollbackListOutput{})
		return
	}

	dir, err := os.Open(updateDir)
	if err != nil {
		printJSON(model.RollbackListOutput{})
		return
	}
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		printJSON(model.RollbackListOutput{})
		return
	}

	versionInfo, _ := config.ReadVersion()
	currentVersion := ""
	if versionInfo != nil {
		currentVersion = versionInfo.Version
	}

	var versions []string
	for _, name := range names {
		subDirPath := filepath.Join(updateDir, name)
		if info, err := os.Stat(subDirPath); err == nil && info.IsDir() {
			versions = append(versions, name)
		}
	}

	printJSON(model.RollbackListOutput{
		CurrentVersion: currentVersion,
		Versions:       versions,
	})
}

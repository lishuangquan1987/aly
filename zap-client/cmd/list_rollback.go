package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"zap/zap-client-sdk/config"
	"zap/zap-client-sdk/model"
)

// ListRollbackVersions lists available rollback versions
func ListRollbackVersions() {
	fs := flag.NewFlagSet("list_rollback_versions", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig("", "", *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	pkgDir, err := config.PackageDir()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	folderName, err := fc.ExeCfg.MainExeFolderName()
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	dir, err := os.Open(pkgDir)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	defer dir.Close()

	names, err := dir.Readdirnames(0)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	prefix := folderName + "_"
	versionInfo, _ := config.ReadVersion()
	currentVersion := ""
	if versionInfo != nil {
		currentVersion = versionInfo.Version
	}

	var versions []string
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		version := strings.TrimPrefix(name, prefix)
		if !isLikelyVersion(version) {
			continue
		}
		subDirPath := filepath.Join(pkgDir, name)
		if info, statErr := os.Stat(subDirPath); statErr == nil && info.IsDir() {
			versions = append(versions, version)
		}
	}

	printOutput(true, "", &model.RollbackListData{
		CurrentVersion: currentVersion,
		Versions:       versions,
	})
}

// isLikelyVersion checks if a string looks like a version number (e.g., "1.0.0")
func isLikelyVersion(s string) bool {
	if len(s) == 0 || len(s) > 50 {
		return false
	}
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if len(part) == 0 {
			return false
		}
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				return false
			}
		}
	}
	return true
}

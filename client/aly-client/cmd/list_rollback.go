package cmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
)

// ListRollbackVersions lists available rollback versions
func ListRollbackVersions() {
	fs := flag.NewFlagSet("list_rollback_versions", flag.ContinueOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	// A6：ContinueOnError + JSON 信封，参数解析失败 exit 2；-h 打印 usage 后 exit 0
	parseCommandFlags(fs, os.Args[2:])

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
	pendingVersion := ""
	if versionInfo != nil {
		// #12 口径统一：version.json 可能带 v 前缀（服务端允许），列表输出
		// 与目录名解析出的版本（无前缀）保持同一口径。
		currentVersion = stripVPrefix(versionInfo.Version)
		if versionInfo.VersionStatus == config.VersionStatusDownloaded {
			// downloaded 状态：主程序仍是 VersionPrevious 的内容，Version 是待应用版本
			if versionInfo.VersionPrevious != "" {
				currentVersion = stripVPrefix(versionInfo.VersionPrevious)
			}
			// 待应用版本目录是下载产物，不可作为回滚目标，列表需排除
			// （目录名解析出的 version 无前缀，此处同样 strip 后比较）
			pendingVersion = stripVPrefix(versionInfo.Version)
		}
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
		if version == pendingVersion {
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

// isLikelyVersion checks if a string looks like a version number.
// 支持纯数字版本（1.0.0）与 semver 预发布/构建元数据（1.0.0-beta、1.0.0+build、1.0.0.1）。
func isLikelyVersion(s string) bool {
	if len(s) == 0 || len(s) > 50 {
		return false
	}
	// 去掉构建元数据与预发布后缀，检查主版本号部分
	core := s
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core = core[:i]
	}
	if i := strings.IndexByte(core, '-'); i >= 0 {
		core = core[:i]
	}
	if core == "" {
		return false
	}
	parts := strings.Split(core, ".")
	// #12 口径统一：单段版本（如 "2"）与多段（"1.0.0"）同为合法——
	// 服务端 PublishVersion 现按 ^v?\d+(\.\d+)*… 校验，允许单段；
	// 此处若仍要求 len(parts) >= 2，单段版本发布会生成快照目录但
	// 永远进不了回滚列表（isLikelyVersion 拒绝）。
	if len(parts) < 1 {
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

package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	apiclient "aly/client/aly-client/client"
	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

// CheckDiff lists files that differ from server (only server-side files)
func CheckDiff() {
	fs := flag.NewFlagSet("check_diff", flag.ContinueOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	// A6：ContinueOnError + JSON 信封，参数解析失败 exit 2；-h 打印 usage 后 exit 0
	parseCommandFlags(fs, os.Args[2:])

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
		latestLog := findLatestLog(logs)
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
		// M12：服务端下发的相对路径必须先过白名单，非法路径不参与比对（并告警留痕），
		// 避免 filepath.Join 被 `..` 引出主目录（与下载/后置脚本同一口径）。
		if !safeRelPath(serverFiles[i].FileRelativePath) {
			exeDir, _ := config.ExeDir()
			util.AppendToLog(exeDir, "check_diff.log",
				fmt.Sprintf("skip unsafe relative path from server: %q", serverFiles[i].FileRelativePath))
			continue
		}
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
	// 并发约束：每个 goroutine 只按下标写 diffFiles[idx]（不同内存地址，非 data race）；
	// 禁止在此循环内对 diffFiles 执行 append/扩容，否则将构成 data race（四-3）。
	var shaWg sync.WaitGroup
	for i := range diffFiles {
		if diffFiles[i].LocalMD5 == "" {
			continue // file doesn't exist locally, no SHA256 needed
		}
		shaWg.Add(1)
		go func(idx int) {
			defer shaWg.Done()
			localFullPath := filepath.Join(fc.MainFolder, filepathFromSlash(diffFiles[idx].Path))
			sha256, shaErr := util.FileSHA256(localFullPath)
			// 轻微项：SHA256 计算失败不再静默吞掉（原先读失败输出空串，排障误导）。
			// 单文件失败不中断整体比对：留痕后置空串，调用方按"无法计算"看待。
			if shaErr != nil {
				exeDir, _ := config.ExeDir()
				util.AppendToLog(exeDir, "check_diff.log",
					fmt.Sprintf("local sha256 failed for %s: %v", diffFiles[idx].Path, shaErr))
				sha256 = ""
			}
			diffFiles[idx].LocalSHA256 = sha256
		}(i)
	}
	shaWg.Wait()

	printOutput(true, "", &model.CheckDiffData{
		NewVersion: newVersion,
		Files:      diffFiles,
	})
}

package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiclient "clientupdator/client/client"
	"clientupdator/client/config"
	"clientupdator/client/model"
	"clientupdator/client/util"
)

// DownloadUpdate 下载更新
func DownloadUpdate() {
	fs := flag.NewFlagSet("download_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "服务器地址")
	projectNameFlag := fs.String("project-name", "", "项目名称")
	mainExePathFlag := fs.String("main-exe-path", "", "主程序相对路径")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, *mainExePathFlag, "")

	if cfg.URL == "" {
		fmt.Println("false:未配置服务器地址")
		return
	}
	if cfg.ProjectName == "" {
		fmt.Println("false:未配置项目名称")
		return
	}

	// 1. 查找项目
	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}

	// 2. 获取变更日志，找到最新版本
	logs, err := apiclient.GetProjectChangeLogs(cfg.URL, project.ID)
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}
	if len(logs) == 0 {
		fmt.Println("false:服务端无变更日志")
		return
	}
	latestLog := logs[0]
	for i := 1; i < len(logs); i++ {
		if logs[i].ID > latestLog.ID {
			latestLog = logs[i]
		}
	}

	newVersion := stripVPrefix(latestLog.Version)

	// 3. 读取本地版本信息
	versionInfo, err := config.ReadVersion()
	if err != nil {
		fmt.Printf("false:读取版本信息失败: %v\n", err)
		return
	}
	currentVersion := stripVPrefix(versionInfo.Version)

	if newVersion == currentVersion {
		fmt.Println("false:当前已是最新版本")
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}

	// 4. 获取服务端文件列表
	serverFiles, err := apiclient.GetAllFiles(cfg.URL, project.ID)
	if err != nil {
		fmt.Printf("false:获取文件列表失败: %v\n", err)
		return
	}

	// 5. 计算本地文件 MD5 映射
	localMD5Map, err := util.LocalFileMD5Map(mainFolder)
	if err != nil {
		fmt.Printf("false:计算本地文件 MD5 失败: %v\n", err)
		return
	}

	// 6. 找出需要下载的差异文件
	updateDir := filepath.Join(mainFolder, "update", newVersion)
	if err := util.EnsureDir(updateDir); err != nil {
		fmt.Printf("false:创建更新目录失败: %v\n", err)
		return
	}

	var diffFiles []model.FileInfo
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		localMD5, localExists := localMD5Map[relPath]
		if !localExists || localMD5 != serverFiles[i].MD5 {
			diffFiles = append(diffFiles, serverFiles[i])
		}
	}

	// 7. 下载差异文件（带 MD5 校验重试）
	fmt.Fprintf(os.Stderr, "开始下载 %d 个差异文件...\n", len(diffFiles))
	for i := range diffFiles {
		serverFile := diffFiles[i]
		localPath := filepath.Join(updateDir, normalizePathLocal(serverFile.FileRelativePath))

		if err := downloadWithRetry(cfg.URL, serverFile.FileAbsolutePath, localPath, serverFile.MD5, 3); err != nil {
			fmt.Printf("false:下载文件 %s 失败: %v\n", serverFile.FileRelativePath, err)
			return
		}
		fmt.Fprintf(os.Stderr, "已下载: %s\n", serverFile.FileRelativePath)
	}

	// 8. 将主程序目录文件复制到 new_version 目录（不覆盖已存在的）
	fmt.Fprintf(os.Stderr, "复制本地文件到更新目录...\n")
	if err := util.CopyDir(mainFolder, updateDir, false); err != nil {
		fmt.Printf("false:复制本地文件失败: %v\n", err)
		return
	}

	// 9. 备份当前版本
	currentVersionDir := filepath.Join(mainFolder, "update", currentVersion)
	fmt.Fprintf(os.Stderr, "备份当前版本到 %s...\n", currentVersionDir)
	if err := util.CopyDir(mainFolder, currentVersionDir, true); err != nil {
		fmt.Printf("false:备份当前版本失败: %v\n", err)
		return
	}

	// 10. 更新 version.json
	versionInfo.Version = newVersion
	versionInfo.VersionStatus = config.VersionStatusDownloaded
	if err := config.WriteVersion(versionInfo); err != nil {
		fmt.Printf("false:更新版本信息失败: %v\n", err)
		return
	}

	fmt.Println("true")
}

// downloadWithRetry 下载文件并校验 MD5，失败时重试
func downloadWithRetry(serverURL, serverFilePath, localPath, expectedMD5 string, maxRetries int) error {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := apiclient.DownloadFile(serverURL, serverFilePath, localPath); err != nil {
			if attempt < maxRetries {
				fmt.Fprintf(os.Stderr, "下载失败，重试 %d/%d: %v\n", attempt, maxRetries, err)
				continue
			}
			return err
		}

		// 校验 MD5
		md5, err := util.FileMD5(localPath)
		if err != nil {
			if attempt < maxRetries {
				fmt.Fprintf(os.Stderr, "MD5 计算失败，重试 %d/%d: %v\n", attempt, maxRetries, err)
				os.Remove(localPath)
				continue
			}
			return fmt.Errorf("MD5 计算失败: %v", err)
		}

		if md5 != expectedMD5 {
			if attempt < maxRetries {
				fmt.Fprintf(os.Stderr, "MD5 校验失败（期望: %s, 实际: %s），重试 %d/%d\n", expectedMD5, md5, attempt, maxRetries)
				os.Remove(localPath)
				continue
			}
			return fmt.Errorf("MD5 校验失败（期望: %s, 实际: %s）", expectedMD5, md5)
		}

		return nil
	}

	return fmt.Errorf("下载失败，已重试 %d 次", maxRetries)
}

// normalizePathLocal 将路径分隔符转换为本地格式
func normalizePathLocal(path string) string {
	return filepath.FromSlash(normalizePath(path))
}

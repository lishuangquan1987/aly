package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "clientupdator/client/client"
	"clientupdator/client/config"
	"clientupdator/client/model"
	"clientupdator/client/util"
)

// CheckDiff 文件比对
func CheckDiff() {
	fs := flag.NewFlagSet("check_diff", flag.ExitOnError)
	urlFlag := fs.String("url", "", "服务器地址")
	projectNameFlag := fs.String("project-name", "", "项目名称")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, "", "")

	if cfg.URL == "" {
		fmt.Fprintf(os.Stderr, "未配置服务器地址\n")
		return
	}
	if cfg.ProjectName == "" {
		fmt.Fprintf(os.Stderr, "未配置项目名称\n")
		return
	}

	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}

	serverFiles, err := apiclient.GetAllFiles(cfg.URL, project.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取服务端文件列表失败: %v\n", err)
		return
	}

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取主程序目录失败: %v\n", err)
		return
	}

	// 获取本地文件的 MD5 映射
	localMD5Map, err := util.LocalFileMD5Map(mainFolder)
	if err != nil {
		fmt.Fprintf(os.Stderr, "计算本地文件 MD5 失败: %v\n", err)
		return
	}

	// 构建服务端文件映射 (relativePath -> FileInfo)
	serverFileMap := make(map[string]model.FileInfo)
	for i := range serverFiles {
		// 将服务端路径分隔符统一为 /
		relPath := normalizePath(serverFiles[i].FileRelativePath)
		serverFileMap[relPath] = serverFiles[i]
	}

	// 找出差异
	var diffs []model.FileDiff

	// 服务端有但本地没有或不同的文件
	for relPath, serverFileInfo := range serverFileMap {
		localMD5, localExists := localMD5Map[relPath]
		if !localExists {
			diffs = append(diffs, model.FileDiff{
				RelativePath: relPath,
				DiffType:     model.DiffTypeServerOnly,
				LocalSize:    -1,
				ServerSize:   serverFileInfo.FileSize,
				LocalMD5:     "-",
				ServerMD5:    serverFileInfo.MD5,
			})
		} else if localMD5 != serverFileInfo.MD5 {
			localSize, _ := util.FileSize(getLocalPath(mainFolder, relPath))
			diffs = append(diffs, model.FileDiff{
				RelativePath: relPath,
				DiffType:     model.DiffTypeModified,
				LocalSize:    localSize,
				ServerSize:   serverFileInfo.FileSize,
				LocalMD5:     localMD5,
				ServerMD5:    serverFileInfo.MD5,
			})
		}
	}

	// 本地有但服务端没有的文件
	for relPath, localMD5 := range localMD5Map {
		normalizedPath := normalizePath(relPath)
		if _, serverExists := serverFileMap[normalizedPath]; !serverExists {
			localSize, _ := util.FileSize(getLocalPath(mainFolder, relPath))
			diffs = append(diffs, model.FileDiff{
				RelativePath: relPath,
				DiffType:     model.DiffTypeLocalOnly,
				LocalSize:    localSize,
				ServerSize:   -1,
				LocalMD5:     localMD5,
				ServerMD5:    "-",
			})
		}
	}

	// 输出差异
	for i := range diffs {
		diff := diffs[i]
		fmt.Printf("%s\t%d\t%d\t%s\t%s\n",
			diff.RelativePath, diff.LocalSize, diff.ServerSize,
			diff.LocalMD5, diff.ServerMD5)
	}
}

func getLocalPath(root, relPath string) string {
	return root + string(os.PathSeparator) + relPath
}

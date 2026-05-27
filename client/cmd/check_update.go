package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "clientupdator/client/client"
	"clientupdator/client/config"
)

// CheckUpdate 检查更新
func CheckUpdate() {
	fs := flag.NewFlagSet("check_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "服务器地址")
	projectNameFlag := fs.String("project-name", "", "项目名称")
	fs.Parse(os.Args[2:])

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}
	cfg.MergeFlags(*urlFlag, *projectNameFlag, "", "")

	if cfg.URL == "" {
		fmt.Println("false:未配置服务器地址")
		return
	}
	if cfg.ProjectName == "" {
		fmt.Println("false:未配置项目名称")
		return
	}

	project, err := apiclient.FindProjectByName(cfg.URL, cfg.ProjectName)
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}

	logs, err := apiclient.GetProjectChangeLogs(cfg.URL, project.ID)
	if err != nil {
		fmt.Printf("false:%v\n", err)
		return
	}

	if len(logs) == 0 {
		fmt.Println("false:服务端无变更日志")
		return
	}

	// 找到最新版本（ID 最大的变更日志）
	latestLog := logs[0]
	for i := 1; i < len(logs); i++ {
		if logs[i].ID > latestLog.ID {
			latestLog = logs[i]
		}
	}

	// 读取本地版本信息
	versionInfo, err := config.ReadVersion()
	if err != nil {
		fmt.Printf("false:读取版本信息失败: %v\n", err)
		return
	}

	// 去掉前缀 V 进行比较（服务端版本可能是 "V1.0.0"，本地版本可能是 "1.0.0"）
	serverVersion := stripVPrefix(latestLog.Version)
	localVersion := stripVPrefix(versionInfo.Version)

	if serverVersion != localVersion {
		fmt.Printf("true:%s\n", serverVersion)
	} else {
		fmt.Println("false:")
	}
}

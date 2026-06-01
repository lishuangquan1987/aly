package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"publish-cli/internal/api"
	"publish-cli/internal/config"
	"publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

var (
	serverURL   string
	projectName string
	projectPath string
	projectID   int
	jsonOutput  bool
	quiet       bool
)

// printOutput 按 isSuccess/errorMsg/data 格式输出 JSON 到 stdout
func printOutput(success bool, errMsg string, data interface{}) {
	out := models.Output{
		IsSuccess: success,
		ErrMsg:    errMsg,
		Data:      data,
	}
	bytes, _ := json.Marshal(out)
	fmt.Println(string(bytes))
}

// printHuman 格式化输出
func printHuman(format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Printf(format, args...)
}

// printHumanLn 格式化输出并换行
func printHumanLn(format string, args ...interface{}) {
	if quiet {
		return
	}
	fmt.Printf(format+"\n", args...)
}

// resolveConfig 解析配置（命令行参数覆盖配置文件）
func resolveConfig() (config.Config, error) {
	var cfg config.Config
	if projectPath != "" {
		cfg, _ = config.LoadProject(projectPath)
	} else {
		cfg, _ = config.LoadGlobal()
	}
	// 命令行覆盖
	if serverURL != "" {
		cfg.Server.URL = serverURL
	}
	if projectName != "" {
		cfg.Project.Name = projectName
	}
	if projectPath != "" {
		cfg.Project.Path = projectPath
	}
	if projectID != 0 {
		cfg.Project.ID = projectID
	}
	return cfg, nil
}

// requireServer checks that server URL is configured
func requireServer(cfg *config.Config) error {
	if cfg.Server.URL == "" {
		return fmt.Errorf("no server URL configured; use --server or config set server.url")
	}
	return nil
}

// requireProject checks that project name and path are configured
func requireProject(cfg *config.Config) error {
	if cfg.Project.Name == "" {
		return fmt.Errorf("no project name configured")
	}
	if cfg.Project.Path == "" {
		return fmt.Errorf("no project path configured")
	}
	return nil
}

// newAPIClient creates an API client from config
func newAPIClient(cfg config.Config) *api.Client {
	return api.NewClient(cfg.Server.URL)
}

// resolveProjectID 从配置中获取项目 ID，若为 0 则按名称查找
func resolveProjectID(cfg config.Config) (int, error) {
	if cfg.Project.ID != 0 {
		return cfg.Project.ID, nil
	}
	if cfg.Project.Name == "" || cfg.Server.URL == "" {
		return 0, fmt.Errorf("需要 --id 或 --project + --server")
	}
	client := api.NewClient(cfg.Server.URL)
	projects, err := client.GetAllProjects()
	if err != nil {
		return 0, fmt.Errorf("获取项目列表失败: %w", err)
	}
	for _, p := range projects {
		if p.Name == cfg.Project.Name {
			return p.ID, nil
		}
	}
	return 0, fmt.Errorf("项目 '%s' 不存在", cfg.Project.Name)
}

// outputResult 根据输出模式输出结果
func outputResult(success bool, errMsg string, data interface{}) {
	if jsonOutput {
		printOutput(success, errMsg, data)
	} else if !success {
		fmt.Fprintf(os.Stderr, "Error: %s\n", errMsg)
	}
}

// RootCmd 根命令
var RootCmd = &cobra.Command{
	Use:   "publish-cli",
	Short: "命令行发布工具 — 将本地构建产物推送到服务端",
	Long: `publish-cli 是面向发布者的命令行工具，用于管理项目、对比文件差异、
暂存变更文件、推送新版本到服务端。

工作流：
  publish-cli config init    # 初始化项目配置
  publish-cli status          # 查看本地与服务端差异
  publish-cli add --all       # 暂存所有变更
  publish-cli push --version V1.0.1 --message "更新说明"  # 推送并发布`,
}

func init() {
	RootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "服务器地址")
	RootCmd.PersistentFlags().StringVar(&projectName, "project", "", "项目名称")
	RootCmd.PersistentFlags().StringVar(&projectPath, "path", "", "本地构建产物路径")
	RootCmd.PersistentFlags().IntVar(&projectID, "id", 0, "项目ID")
	RootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON 格式输出")
	RootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "静默模式")
}

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

// RuntimeConfig 运行时配置（CLI 参数 + 配置文件合并结果）
type RuntimeConfig struct {
	Shared  config.SharedConfig
	Publish config.PublishConfig
	Path    string // 项目路径（--path CLI 参数）
	ID      int    // 项目 ID（--id CLI 参数）
}

// printOutput 按 isSuccess/errorMsg/data 格式输出 JSON 到 stdout
func printOutput(success bool, errMsg string, data interface{}) {
	out := models.Output{
		IsSuccess: success,
		ErrMsg:    errMsg,
		Data:      data,
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		os.Exit(1)
	}
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

// resolveConfig 解析配置（.updator/ 文件 + CLI 参数覆盖）
// 必须指定 --path，否则报错。
func resolveConfig() (RuntimeConfig, error) {
	if projectPath == "" {
		return RuntimeConfig{}, fmt.Errorf("请指定 --path")
	}

	shared, err := config.LoadShared(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load shared.json: %v\n", err)
		shared = config.DefaultShared()
	}
	publish, err := config.LoadPublish(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load publish.json: %v\n", err)
		publish = config.DefaultPublish()
	}

	// CLI 参数覆盖配置文件
	if serverURL != "" {
		shared.ServerURL = serverURL
	}
	if projectName != "" {
		shared.ProjectName = projectName
	}

	return RuntimeConfig{
		Shared:  shared,
		Publish: publish,
		Path:    projectPath,
		ID:      projectID,
	}, nil
}

// requireServer checks that server URL is configured
func requireServer(cfg *RuntimeConfig) error {
	if cfg.Shared.ServerURL == "" {
		return fmt.Errorf("no server URL configured; use --server or config set server.url")
	}
	return nil
}

// requireProject checks that project name is configured
func requireProject(cfg *RuntimeConfig) error {
	if cfg.Shared.ProjectName == "" {
		return fmt.Errorf("no project name configured")
	}
	return nil
}

// newAPIClient creates an API client from config
func newAPIClient(cfg RuntimeConfig) *api.Client {
	return api.NewClient(cfg.Shared.ServerURL)
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
	Short: "命令行发布工具——将本地构建产物推送到服务端",
	Long: `publish-cli 向发布的命令行工具，用于管理项目、比文件差异、暂存变更文件、推送新版本到服务端。
工作流：
  publish-cli config init    # 初始化项目
  publish-cli status          # 查看与服务端差异
  publish-cli add --all       # 暂存所有变更
  publish-cli push --version V1.0.1 --message "更新说明"  # 推送并发布`,
}

func init() {
	RootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "服务器地址")
	RootCmd.PersistentFlags().StringVar(&projectName, "project", "", "项目名称")
	RootCmd.PersistentFlags().StringVar(&projectPath, "path", "", "本地构建产物路径（必填）")
	RootCmd.PersistentFlags().IntVar(&projectID, "id", 0, "项目ID（直传，跳过名称查找）")
	RootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON 格式输出")
	RootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "静默模式")
}

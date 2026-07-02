package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"aly/publish-cli/internal/api"
	"aly/publish-cli/internal/config"
	"aly/publish-cli/internal/staging"
	"aly/publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

var (
	serverURL   string
	projectName string
	jsonOutput  bool
	quiet       bool
)

// RuntimeConfig 运行时配置（CLI 参数 + 配置文件合并结果）
type RuntimeConfig struct {
	Shared  config.SharedConfig
	Publish config.PublishConfig
	Path    string // 当前工作目录（由调用者通过 cd / WorkingDirectory 指定）
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
		// 序列化失败时输出一个固定的错误 JSON，避免 os.Exit 阻止 defer 执行
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		fmt.Println(`{"isSuccess":false,"errorMsg":"internal output error","data":null}`)
		return
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

// resolveConfig 解析配置（从当前工作目录的 .updator/ 文件读取 + CLI 参数覆盖）
// 对标 git，始终使用当前目录（由调用者通过 cd 或 WorkingDirectory 指定）。
func resolveConfig() (RuntimeConfig, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return RuntimeConfig{}, fmt.Errorf("无法获取当前目录: %w", err)
	}

	shared, err := config.LoadShared(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load shared.json: %v\n", err)
		shared = config.DefaultShared()
	}
	publish, err := config.LoadPublish(cwd)
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
		Path:    cwd,
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

// printProgress 输出上传进度到 stdout（仅 --json 模式），每行一个 JSON。
// data 中包含 index/total/file/status/file_size/error 字段。
func printProgress(index, total int, file, status string, fileSize int64, errMsg string) {
	if !jsonOutput {
		return
	}
	out := models.Output{
		IsSuccess: true,
		ErrMsg:    "",
		Data: models.UploadProgress{
			Index:    index,
			Total:    total,
			File:     file,
			Status:   status,
			FileSize: fileSize,
			Error:    errMsg,
		},
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// printProgressFail 输出失败的进度行（isSuccess: false）。
func printProgressFail(index, total int, file string, fileSize int64, errMsg string) {
	if !jsonOutput {
		return
	}
	out := models.Output{
		IsSuccess: false,
		ErrMsg:    fmt.Sprintf("%s: %s", file, errMsg),
		Data: models.UploadProgress{
			Index:    index,
			Total:    total,
			File:     file,
			Status:   "FAIL",
			FileSize: fileSize,
			Error:    errMsg,
		},
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// printProgressDone 输出最终完成行（isSuccess: true, data: null），表示进度流结束。
func printProgressDone() {
	if !jsonOutput {
		return
	}
	out := models.Output{
		IsSuccess: true,
		ErrMsg:    "",
		Data:      nil,
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// mergeStagedIntoStatusData 从暂存区加载 staged 文件，并从 unstaged 中移除已暂存的条目
func mergeStagedIntoStatusData(sd *models.StatusData, projectPath string) {
	stagedItems, err := staging.Load(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load staging: %v\n", err)
		return
	}
	stagedPaths := make(map[string]bool, len(stagedItems))
	for _, s := range stagedItems {
		stagedPaths[s.RelativePath] = true
	}
	sd.Staged = stagedItems

	var filtered []models.FileStatusItem
	for _, u := range sd.Unstaged {
		if !stagedPaths[u.RelativePath] {
			filtered = append(filtered, u)
		}
	}
	sd.Unstaged = filtered
}

// RootCmd 根命令
var RootCmd = &cobra.Command{
	Use:   "aly-publish",
	Short: "命令行发布工具——将本地构建产物推送到服务端",
	Long: `aly-publish 向发布的命令行工具，用于管理项目、比文件差异、暂存变更文件、推送新版本到服务端。
工作流：
  aly-publish config init    # 初始化项目
  aly-publish status          # 查看与服务端差异
  aly-publish add --all       # 暂存所有变更
  aly-publish push --version V1.0.1 --message "更新说明"  # 推送并发布`,
}

func init() {
	RootCmd.PersistentFlags().StringVar(&serverURL, "server", "", "服务器地址")
	RootCmd.PersistentFlags().StringVar(&projectName, "project", "", "项目名称")
	RootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "JSON 格式输出")
	RootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "静默模式")
}

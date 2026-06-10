package cmd

import (
	"fmt"

	"zap/publish-cli/internal/config"

	"github.com/spf13/cobra"
)

var (
	configKey   string
	configValue string
	setArrayAdd    string
	setArrayRemove string
	setArrayClear  bool
)

func init() {
	cmdConfigInit.MarkFlagRequired("project")

	RootCmd.AddCommand(cmdConfig)
	cmdConfig.AddCommand(cmdConfigInit)
	cmdConfig.AddCommand(cmdConfigSet)
	cmdConfig.AddCommand(cmdConfigSetArray)
	cmdConfig.AddCommand(cmdConfigGet)
	cmdConfig.AddCommand(cmdConfigList)
	cmdConfig.AddCommand(cmdConfigPath)
}

var cmdConfig = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
}

var cmdConfigInit = &cobra.Command{
	Use:   "init",
	Short: "初始化项目配置（创建 .updator/shared.json + .updator/publish.json）",
	Run:   runConfigInit,
}

var cmdConfigSet = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置项",
	Run:   runConfigSet,
}

var cmdConfigSetArray = &cobra.Command{
	Use:   "set-array <key> --add <item> | --remove <item> | --clear",
	Short: "设置数组配置项",
	Run:   runConfigSetArray,
}

var cmdConfigGet = &cobra.Command{
	Use:   "get <key>",
	Short: "获取配置项",
	Run:   runConfigGet,
}

var cmdConfigList = &cobra.Command{
	Use:   "list",
	Short: "列出所有配置",
	Run:   runConfigList,
}

var cmdConfigPath = &cobra.Command{
	Use:   "path",
	Short: "显示配置文件路径",
	Run:   runConfigPath,
}

func runConfigInit(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := config.SaveShared(cfg.Path, cfg.Shared); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := config.SavePublish(cfg.Path, cfg.Publish); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", map[string]string{
			"shared_path":  config.SharedPath(cfg.Path),
			"publish_path": config.PublishPath(cfg.Path),
		})
		return
	}
	printHumanLn("配置已保存到: %s", config.UpdatorDir(cfg.Path))
}

func runConfigSet(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		outputResult(false, "Usage: zap-publish config set <key> <value>", nil)
		return
	}
	key := args[0]
	value := args[1]
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	applyConfigSet(&cfg, key, value)
	if err := config.SaveShared(cfg.Path, cfg.Shared); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := config.SavePublish(cfg.Path, cfg.Publish); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", nil)
		return
	}
	printHumanLn("配置已更新 %s = %s", key, value)
}

func runConfigSetArray(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		outputResult(false, "Usage: zap-publish config set-array <key> --add <item> | --remove <item> | --clear", nil)
		return
	}
	key := args[0]
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	applyArrayOp(&cfg, key, setArrayAdd, setArrayRemove, setArrayClear)
	if err := config.SaveShared(cfg.Path, cfg.Shared); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := config.SavePublish(cfg.Path, cfg.Publish); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", nil)
		return
	}
	printHumanLn("配置已更新 %s", key)
}

func runConfigGet(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: zap-publish config get <key>")
		return
	}
	key := args[0]
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	val := getConfigValue(&cfg, key)
	if jsonOutput {
		printOutput(true, "", map[string]string{key: val})
		return
	}
	fmt.Println(val)
}

func runConfigList(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", map[string]interface{}{
			"shared":  cfg.Shared,
			"publish": cfg.Publish,
		})
		return
	}
	printHumanLn("server.url      = %s", cfg.Shared.ServerURL)
	printHumanLn("project.name    = %s", cfg.Shared.ProjectName)
	printHumanLn("ignore.folders  = %v", cfg.Shared.IgnoreFolders)
	printHumanLn("ignore.files    = %v", cfg.Shared.IgnoreFiles)
	printHumanLn("output.format   = %s", cfg.Publish.OutputFormat)
}

func runConfigPath(cmd *cobra.Command, args []string) {
	cfg, _ := resolveConfig()
	fmt.Println(config.UpdatorDir(cfg.Path))
}

//  helpers 

func applyConfigSet(cfg *RuntimeConfig, key, value string) {
	switch key {
	case "server.url":
		cfg.Shared.ServerURL = value
	case "project.name":
		cfg.Shared.ProjectName = value
	case "output.format":
		cfg.Publish.OutputFormat = value
	default:
		fmt.Printf("Warning: unknown config key '%s'\n", key)
	}
}

func applyArrayOp(cfg *RuntimeConfig, key, add, remove string, clear bool) {
	switch key {
	case "ignore.folders":
		if clear {
			cfg.Shared.IgnoreFolders = nil
		}
		if add != "" {
			cfg.Shared.IgnoreFolders = append(cfg.Shared.IgnoreFolders, add)
		}
		if remove != "" {
			var filtered []string
			for _, f := range cfg.Shared.IgnoreFolders {
				if f != remove {
					filtered = append(filtered, f)
				}
			}
			cfg.Shared.IgnoreFolders = filtered
		}
	case "ignore.files":
		if clear {
			cfg.Shared.IgnoreFiles = nil
		}
		if add != "" {
			cfg.Shared.IgnoreFiles = append(cfg.Shared.IgnoreFiles, add)
		}
		if remove != "" {
			var filtered []string
			for _, f := range cfg.Shared.IgnoreFiles {
				if f != remove {
					filtered = append(filtered, f)
				}
			}
			cfg.Shared.IgnoreFiles = filtered
		}
	}
}

func getConfigValue(cfg *RuntimeConfig, key string) string {
	switch key {
	case "server.url":
		return cfg.Shared.ServerURL
	case "project.name":
		return cfg.Shared.ProjectName
	case "output.format":
		return cfg.Publish.OutputFormat
	default:
		return ""
	}
}

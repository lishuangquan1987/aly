package cmd

import (
	"fmt"

	"aly/publish-cli/internal/config"

	"github.com/spf13/cobra"
)

var (
	configKey      string
	configValue    string
	setArrayAdd    string
	setArrayRemove string
	setArrayClear  bool

	ignoreFoldersInit  string
	ignoreFilesInit    string
	unCopyFoldersInit  string
	unCopyFilesInit    string
)

func init() {
	cmdConfigInit.Flags().StringVar(&ignoreFoldersInit, "ignore-folders", "", "忽略的文件夹（逗号分隔）")
	cmdConfigInit.Flags().StringVar(&ignoreFilesInit, "ignore-files", "", "忽略的文件（逗号分隔）")
	cmdConfigInit.Flags().StringVar(&unCopyFoldersInit, "un-copy-folders", "", "不复制文件夹（逗号分隔）")
	cmdConfigInit.Flags().StringVar(&unCopyFilesInit, "un-copy-files", "", "不复制文件（逗号分隔）")
	cmdConfigInit.MarkFlagRequired("project")

	cmdConfigSetArray.Flags().StringVar(&setArrayAdd, "add", "", "添加项")
	cmdConfigSetArray.Flags().StringVar(&setArrayRemove, "remove", "", "移除项")
	cmdConfigSetArray.Flags().BoolVar(&setArrayClear, "clear", false, "清空")

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
	// 应用 --ignore-folders 和 --ignore-files（逗号分隔）
	if ignoreFoldersInit != "" {
		cfg.Shared.IgnoreFolders = parseCSV(ignoreFoldersInit)
	}
	if ignoreFilesInit != "" {
		cfg.Shared.IgnoreFiles = parseCSV(ignoreFilesInit)
	}
	// 应用 --un-copy-folders 和 --un-copy-files（逗号分隔）
	if unCopyFoldersInit != "" {
		cfg.Shared.UnCopyFolders = parseCSV(unCopyFoldersInit)
	}
	if unCopyFilesInit != "" {
		cfg.Shared.UnCopyFiles = parseCSV(unCopyFilesInit)
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
		outputResult(false, "Usage: aly-publish config set <key> <value>", nil)
		return
	}
	key := args[0]
	value := args[1]
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := applyConfigSet(&cfg, key, value); err != nil {
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
		printOutput(true, "", nil)
		return
	}
	printHumanLn("配置已更新 %s = %s", key, value)
}

func runConfigSetArray(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		outputResult(false, "Usage: aly-publish config set-array <key> --add <item> | --remove <item> | --clear", nil)
		return
	}
	key := args[0]
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := applyArrayOp(&cfg, key, setArrayAdd, setArrayRemove, setArrayClear); err != nil {
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
		printOutput(true, "", nil)
		return
	}
	printHumanLn("配置已更新 %s", key)
}

func runConfigGet(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		outputResult(false, "Usage: aly-publish config get <key>", nil)
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
	printHumanLn("%s", val)
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
	printHumanLn("ignore.folders   = %v", cfg.Shared.IgnoreFolders)
	printHumanLn("ignore.files     = %v", cfg.Shared.IgnoreFiles)
	printHumanLn("un_copy.folders  = %v", cfg.Shared.UnCopyFolders)
	printHumanLn("un_copy.files    = %v", cfg.Shared.UnCopyFiles)

}

func runConfigPath(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", map[string]string{"path": config.UpdatorDir(cfg.Path)})
		return
	}
	printHumanLn("%s", config.UpdatorDir(cfg.Path))
}

//  helpers

func applyConfigSet(cfg *RuntimeConfig, key, value string) error {
	switch key {
	case "server.url":
		cfg.Shared.ServerURL = value
	case "project.name":
		cfg.Shared.ProjectName = value
	default:
		return fmt.Errorf("unknown config key '%s'", key)
	}
	return nil
}

func applyArrayOp(cfg *RuntimeConfig, key, add, remove string, clear bool) error {
	var target *[]string
	switch key {
	case "ignore.folders":
		target = &cfg.Shared.IgnoreFolders
	case "ignore.files":
		target = &cfg.Shared.IgnoreFiles
	case "un_copy.folders":
		target = &cfg.Shared.UnCopyFolders
	case "un_copy.files":
		target = &cfg.Shared.UnCopyFiles
	default:
		return fmt.Errorf("unknown config key '%s'", key)
	}
	applyStringSliceOp(target, add, remove, clear)
	return nil
}

// applyStringSliceOp 对字符串切片执行 clear / add / remove 操作
func applyStringSliceOp(target *[]string, add, remove string, clear bool) {
	if target == nil {
		return
	}
	if clear {
		*target = nil
	}
	if add != "" {
		*target = append(*target, add)
	}
	if remove != "" {
		var filtered []string
		for _, f := range *target {
			if f != remove {
				filtered = append(filtered, f)
			}
		}
		*target = filtered
	}
}

func getConfigValue(cfg *RuntimeConfig, key string) string {
	switch key {
	case "server.url":
		return cfg.Shared.ServerURL
	case "project.name":
		return cfg.Shared.ProjectName
	default:
		return ""
	}
}

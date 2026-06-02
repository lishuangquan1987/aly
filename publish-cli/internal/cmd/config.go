package cmd

import (
	"fmt"
	"os"

	"publish-cli/internal/config"

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
	cmdConfigInit.Flags().IntVar(&projectID, "id", 0, "项目ID")
	cmdConfigInit.MarkFlagRequired("project")
	cmdConfigInit.MarkFlagRequired("path")

	cmdConfigSet.Flags().StringVar(&configKey, "key", "", "配置键")
	cmdConfigSet.Flags().StringVar(&configValue, "value", "", "配置值")

	cmdConfigSetArray.Flags().StringVar(&configKey, "key", "", "配置键")
	cmdConfigSetArray.Flags().StringVar(&setArrayAdd, "add", "", "添加项")
	cmdConfigSetArray.Flags().StringVar(&setArrayRemove, "remove", "", "移除项")
	cmdConfigSetArray.Flags().BoolVar(&setArrayClear, "clear", false, "清空数组")

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
	Short: "初始化项目配置",
	Run:   runConfigInit,
}

var cmdConfigSet = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "设置配置项（标量值）",
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
	cfg.Project.ID = projectID
	if err := config.SaveProject(projectPath, cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", cfg)
		return
	}
	printHumanLn("项目配置已保存到: %s", config.ProjectPath(projectPath))
}

func runConfigSet(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		if jsonOutput {
			printOutput(false, "Usage: publish-cli config set <key> <value>", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Usage: publish-cli config set <key> <value>")
		}
		return
	}
	key := args[0]
	value := args[1]
	cfg, _ := resolveConfig()
	applyConfigSet(&cfg, key, value)
	if projectPath != "" {
		if err := config.SaveProject(projectPath, cfg); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
	} else {
		if err := config.SaveGlobal(cfg); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
	}
	if jsonOutput {
		printOutput(true, "", nil)
		return
	}
	printHumanLn("配置已更新: %s = %s", key, value)
}

func runConfigSetArray(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		if jsonOutput {
			printOutput(false, "Usage: publish-cli config set-array <key> --add <item> | --remove <item> | --clear", nil)
		} else {
			fmt.Fprintln(os.Stderr, "Usage: publish-cli config set-array <key> --add <item> | --remove <item> | --clear")
		}
		return
	}
	key := args[0]
	cfg, _ := resolveConfig()
	if projectPath != "" {
		config.LoadProject(projectPath) // ensure project config exists
	}
	applyArrayOp(&cfg, key, setArrayAdd, setArrayRemove, setArrayClear)
	if projectPath != "" {
		if err := config.SaveProject(projectPath, cfg); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
	} else {
		if err := config.SaveGlobal(cfg); err != nil {
			outputResult(false, err.Error(), nil)
			return
		}
	}
	if jsonOutput {
		printOutput(true, "", nil)
		return
	}
	printHumanLn("配置已更新: %s", key)
}

func runConfigGet(cmd *cobra.Command, args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: publish-cli config get <key>")
		return
	}
	key := args[0]
	cfg, _ := resolveConfig()
	val := getConfigValue(&cfg, key)
	if jsonOutput {
		printOutput(true, "", map[string]string{key: val})
		return
	}
	fmt.Println(val)
}

func runConfigList(cmd *cobra.Command, args []string) {
	cfg, _ := resolveConfig()
	if jsonOutput {
		printOutput(true, "", cfg)
		return
	}
	printHumanLn("server.url      = %s", cfg.Server.URL)
	printHumanLn("project.name    = %s", cfg.Project.Name)
	printHumanLn("project.path    = %s", cfg.Project.Path)
	printHumanLn("project.id      = %d", cfg.Project.ID)
	printHumanLn("ignore.folders  = %v", cfg.Ignore.Folders)
	printHumanLn("ignore.files    = %v", cfg.Ignore.Files)
	printHumanLn("output.format   = %s", cfg.Output.Format)
}

func runConfigPath(cmd *cobra.Command, args []string) {
	if projectPath != "" {
		fmt.Println(config.ProjectPath(projectPath))
	} else {
		path, _ := config.GlobalPath()
		fmt.Println(path)
	}
}

// ─── helpers ──────────────────────────────────────────────────────────

func applyConfigSet(cfg *config.Config, key, value string) {
	switch key {
	case "server.url":
		cfg.Server.URL = value
	case "project.name":
		cfg.Project.Name = value
	case "project.path":
		cfg.Project.Path = value
	case "output.format":
		cfg.Output.Format = value
	default:
		fmt.Fprintf(os.Stderr, "Warning: unknown config key '%s'\n", key)
	}
}

func applyArrayOp(cfg *config.Config, key, add, remove string, clear bool) {
	switch key {
	case "ignore.folders":
		if clear {
			cfg.Ignore.Folders = nil
		}
		if add != "" {
			cfg.Ignore.Folders = append(cfg.Ignore.Folders, add)
		}
		if remove != "" {
			var filtered []string
			for _, f := range cfg.Ignore.Folders {
				if f != remove {
					filtered = append(filtered, f)
				}
			}
			cfg.Ignore.Folders = filtered
		}
	case "ignore.files":
		if clear {
			cfg.Ignore.Files = nil
		}
		if add != "" {
			cfg.Ignore.Files = append(cfg.Ignore.Files, add)
		}
		if remove != "" {
			var filtered []string
			for _, f := range cfg.Ignore.Files {
				if f != remove {
					filtered = append(filtered, f)
				}
			}
			cfg.Ignore.Files = filtered
		}
	}
}

func getConfigValue(cfg *config.Config, key string) string {
	switch key {
	case "server.url":
		return cfg.Server.URL
	case "project.name":
		return cfg.Project.Name
	case "project.path":
		return cfg.Project.Path
	case "output.format":
		return cfg.Output.Format
	default:
		return ""
	}
}

package cmd

import (
	"fmt"
	"os"
	"strings"

	"publish-cli/pkg/models"

	"github.com/spf13/cobra"
)

var (
	pForceUpdate     bool
	pTitle           string
	pUpdateName      string
	pIgnoreFoldersStr string
	pIgnoreFilesStr   string
)

func init() {
	// project create flags
	cmdProjectCreate.Flags().StringVar(&pUpdateName, "name", "", "项目名称")
	cmdProjectCreate.Flags().StringVar(&pTitle, "title", "", "项目抬头")
	cmdProjectCreate.Flags().BoolVar(&pForceUpdate, "force-update", false, "是否强制更新")
	cmdProjectCreate.Flags().StringVar(&pIgnoreFoldersStr, "ignore-folders", "", "忽略的文件夹（逗号分隔）")
	cmdProjectCreate.Flags().StringVar(&pIgnoreFilesStr, "ignore-files", "", "忽略的文件（逗号分隔）")
	cmdProjectCreate.MarkFlagRequired("name")
	cmdProjectCreate.MarkFlagRequired("title")

	// project update flags
	cmdProjectUpdate.Flags().StringVar(&pUpdateName, "name", "", "项目名称（必填）")
	cmdProjectUpdate.Flags().StringVar(&pTitle, "title", "", "项目抬头")
	cmdProjectUpdate.Flags().BoolVar(&pForceUpdate, "force-update", false, "是否强制更新")
	cmdProjectUpdate.Flags().StringVar(&pIgnoreFoldersStr, "ignore-folders", "", "忽略的文件夹（逗号分隔）")
	cmdProjectUpdate.Flags().StringVar(&pIgnoreFilesStr, "ignore-files", "", "忽略的文件（逗号分隔）")
	cmdProjectUpdate.MarkFlagRequired("name")
	cmdProjectUpdate.MarkFlagRequired("title")

	// project delete flags
	cmdProjectDelete.Flags().StringVar(&pUpdateName, "name", "", "项目名称（必填）")
	cmdProjectDelete.MarkFlagRequired("name")

	// project info flags (keep --id for convenience)
	cmdProjectInfo.Flags().IntVar(&projectID, "id", 0, "项目ID")
	cmdProjectInfo.Flags().StringVar(&pUpdateName, "name", "", "项目名称")

	RootCmd.AddCommand(cmdProject)
	cmdProject.AddCommand(cmdProjectList)
	cmdProject.AddCommand(cmdProjectCreate)
	cmdProject.AddCommand(cmdProjectUpdate)
	cmdProject.AddCommand(cmdProjectDelete)
	cmdProject.AddCommand(cmdProjectInfo)
}

var cmdProject = &cobra.Command{
	Use:   "project",
	Short: "项目管理",
}

var cmdProjectList = &cobra.Command{
	Use:   "list",
	Short: "列出所有项目",
	Run:   runProjectList,
}

var cmdProjectCreate = &cobra.Command{
	Use:   "create",
	Short: "创建新项目",
	Run:   runProjectCreate,
}

var cmdProjectUpdate = &cobra.Command{
	Use:   "update",
	Short: "更新项目配置（按名称）",
	Run:   runProjectUpdate,
}

var cmdProjectDelete = &cobra.Command{
	Use:   "delete",
	Short: "删除项目（按名称）",
	Run:   runProjectDelete,
}

var cmdProjectInfo = &cobra.Command{
	Use:   "info",
	Short: "查看项目详情（--id 或 --name）",
	Run:   runProjectInfo,
}

func parseCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func runProjectList(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	client := newAPIClient(cfg)
	projects, err := client.GetAllProjects()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", projects)
		return
	}
	printHumanLn("  %-4s %-8s %-10s %-8s %-12s %s", "ID", "NAME", "TITLE", "VERSION", "FORCE UPDATE", "CREATED")
	for _, p := range projects {
		fu := "no"
		if p.ForceUpdate {
			fu = "yes"
		}
		created := p.CreatedAt
		if len(created) > 10 {
			created = created[:10]
		} else if len(created) == 0 {
			created = "-"
		}
		printHumanLn("  %-4d %-8s %-10s %-8s %-12s %s", p.ID, p.Name, p.Title, p.Version, fu, created)
	}
}

func runProjectCreate(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	client := newAPIClient(cfg)
	req := models.CreateProjectRequest{
		Name:          pUpdateName,
		Title:         pTitle,
		IsForceUpdate: pForceUpdate,
		IgnoreFolders: parseCSV(pIgnoreFoldersStr),
		IgnoreFiles:   parseCSV(pIgnoreFilesStr),
	}
	project, err := client.CreateProject(req)
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", project)
		return
	}
	printHumanLn("项目创建成功: [%d] %s (%s)", project.ID, project.Name, project.Version)
}

func runProjectUpdate(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	client := newAPIClient(cfg)
	req := models.UpdateProjectRequest{
		Name:          pUpdateName,
		Title:         pTitle,
		IsForceUpdate: pForceUpdate,
		IgnoreFolders: parseCSV(pIgnoreFoldersStr),
		IgnoreFiles:   parseCSV(pIgnoreFilesStr),
	}
	if err := client.UpdateProject(req); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", nil)
		return
	}
	printHumanLn("项目更新成功: %s", pUpdateName)
}

func runProjectDelete(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	client := newAPIClient(cfg)
	if err := client.DeleteProject(pUpdateName); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if jsonOutput {
		printOutput(true, "", nil)
		return
	}
	printHumanLn("项目已删除: %s", pUpdateName)
}

func runProjectInfo(cmd *cobra.Command, args []string) {
	cfg, err := resolveConfig()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	if err := requireServer(&cfg); err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	client := newAPIClient(cfg)
	projects, err := client.GetAllProjects()
	if err != nil {
		outputResult(false, err.Error(), nil)
		return
	}
	matchName := pUpdateName
	for _, p := range projects {
		if (projectID != 0 && p.ID == projectID) || (matchName != "" && p.Name == matchName) {
			if jsonOutput {
				printOutput(true, "", p)
				return
			}
			printHumanLn("Project: %s", p.Name)
			printHumanLn("Title:   %s", p.Title)
			printHumanLn("Version: %s", p.Version)
			fu := "no"
			if p.ForceUpdate {
				fu = "yes"
			}
			printHumanLn("Force Update: %s", fu)
			created := p.CreatedAt
			if len(created) > 10 {
				created = created[:10]
			} else if len(created) == 0 {
				created = "-"
			}
			printHumanLn("Created: %s", created)
			printHumanLn("")
			printHumanLn("Ignore Folders:")
			for _, f := range p.IgnoreFolders {
				printHumanLn("  %s", f)
			}
			printHumanLn("Ignore Files:")
			for _, f := range p.IgnoreFiles {
				printHumanLn("  %s", f)
			}
			return
		}
	}
	fmt.Fprintf(os.Stderr, "Error: 项目不存在\n")
}

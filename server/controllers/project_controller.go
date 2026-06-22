package controllers

import (
	"fmt"
	"regexp"
	"strings"
	"zap/server/ent"
	"zap/server/ent/project"
	"zap/server/internal/db"
	"zap/server/internal/service"
	"zap/server/models"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/directory"
)

// validProjectName 项目名称合法性：只要能创建为文件夹名即可
// 禁止 Windows/Linux 文件系统非法字符：< > : " / \ | ? *
var validProjectName = regexp.MustCompile(`^[^<>:"/\\|?*]{1,64}$`)

// validateProjectName 校验项目名称：白名单 + Windows 保留名检查
func validateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("项目名称不能为空")
	}
	if !validProjectName.MatchString(name) {
		return fmt.Errorf("项目名称包含非法字符或长度不在1-64范围内")
	}
	// Windows 保留名检查
	reserved := map[string]bool{
		"CON": true, "PRN": true, "AUX": true, "NUL": true,
		"COM1": true, "COM2": true, "COM3": true, "COM4": true,
		"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
		"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
		"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
	}
	upper := strings.ToUpper(name)
	if reserved[upper] {
		return fmt.Errorf("项目名称不能使用系统保留名: %s", name)
	}
	return nil
}

func CreateProject(ctx *gin.Context) {
	var createProjectDto struct {
		Name          string   `json:"name"`
		Title         string   `json:"title"`
		IsForceUpdate bool     `json:"isForceUpdate"`
		IgnoreFolders []string `json:"ignoreFolders"`
		IgnoreFiles   []string `json:"ignoreFiles"`
	}
	// 解析请求体
	if err := ctx.ShouldBindJSON(&createProjectDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	// 校验项目名称
	if err := validateProjectName(createProjectDto.Name); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return
	}

	if createProjectDto.Title == "" {
		ctx.JSON(200, models.NG("项目抬头不能为空"))
		return
	}

	workDir, err := service.GetProjectWorkPath(createProjectDto.Name)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	//判断项目名称是否存在
	_, err = db.Client.Project.Query().Where(project.NameEQ(createProjectDto.Name)).First(ctx)
	if err == nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("项目名称:%s已存在", createProjectDto.Name)))
		return
	}

	if !directory.Exists(workDir) {
		//创建文件夹
		if err := directory.CreateDirectory(workDir); err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
	}

	//插入
	result := service.CreateProjectWithFirstLog(ctx.Request.Context(),
		createProjectDto.Name,
		createProjectDto.Title,
		createProjectDto.IsForceUpdate,
		createProjectDto.IgnoreFolders,
		createProjectDto.IgnoreFiles)

	ctx.JSON(200, result)
}

func UpdateProject(ctx *gin.Context) {
	var updateProjectDto struct {
		Name          string   `json:"name"`
		Title         string   `json:"title"`
		IsForceUpdate bool     `json:"isForceUpdate"`
		IgnoreFolders []string `json:"ignoreFolders"`
		IgnoreFiles   []string `json:"ignoreFiles"`
	}
	if err := ctx.ShouldBindJSON(&updateProjectDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if err := validateProjectName(updateProjectDto.Name); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return
	}

	//判断项目抬头是否为空
	if updateProjectDto.Title == "" {
		ctx.JSON(200, models.NG("项目抬头不能为空"))
		return
	}

	//判断项目是否存在（仅未软删除的项目）
	_, err := db.Client.Project.Query().Where(project.NameEQ(updateProjectDto.Name), project.IsDeletedEQ(false)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			ctx.JSON(200, models.NG("项目不存在"))
		} else {
			ctx.JSON(200, models.NGWithError(err))
		}
		return
	}

	//更新
	result := service.UpdateProject(ctx.Request.Context(),
		updateProjectDto.Name,
		updateProjectDto.Title,
		updateProjectDto.IsForceUpdate,
		updateProjectDto.IgnoreFolders,
		updateProjectDto.IgnoreFiles)

	ctx.JSON(200, result)
}

func DeleteProject(ctx *gin.Context) {
	var projectNameDto struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	if err := validateProjectName(projectNameDto.ProjectName); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return
	}

	ctx.JSON(200, service.DeleteProject(ctx.Request.Context(), projectNameDto.ProjectName))
}

func GetAllProjects(ctx *gin.Context) {
	ctx.JSON(200, service.GetAllProjects(ctx.Request.Context()))
}

func GetProjectByName(ctx *gin.Context) {
	var projectNameDto struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	if err := validateProjectName(projectNameDto.ProjectName); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return
	}

	ctx.JSON(200, service.GetProjectByName(ctx.Request.Context(), projectNameDto.ProjectName))
}

func SetForceUpdate(ctx *gin.Context) {
	var req struct {
		ProjectName string `json:"projectName"`
		ForceUpdate bool   `json:"forceUpdate"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if err := validateProjectName(req.ProjectName); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return
	}
	ctx.JSON(200, service.SetForceUpdate(ctx.Request.Context(), req.ProjectName, req.ForceUpdate))
}

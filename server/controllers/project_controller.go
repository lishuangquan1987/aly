package controllers

import (
	"fmt"
	"aly/server/ent"
	"aly/server/ent/project"
	"aly/server/internal/db"
	"aly/server/internal/service"
	"aly/server/models"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/directory"
)

// validateProjectName 校验项目名称。
//
// 实现已收敛到 service.ValidateProjectName（审查 B1/A7）：上传、下载、发布、
// 系统信息等所有入口共用同一套规则，避免"某个入口漏校验"再次发生。
func validateProjectName(name string) error {
	return service.ValidateProjectName(name)
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
	// #13：用 ent.IsNotFound 精确区分"不存在"与"查询出错"——
	// 旧代码 err != nil 一律当"不存在"继续插入：DB 故障/网络抖动时
	// 唯一性校验被吞掉，插入层才报错（错误信息难定位）。
	_, err = db.Client.Project.Query().Where(project.NameEQ(createProjectDto.Name)).First(ctx)
	if err == nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("项目名称:%s已存在", createProjectDto.Name)))
		return
	}
	if !ent.IsNotFound(err) {
		ctx.JSON(200, models.NGWithError(err))
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

	// 忽略规则变化会影响文件列表过滤：使该项目的文件列表缓存失效
	if result.IsSuccess {
		service.InvalidateProjectFileList(updateProjectDto.Name)
	}

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

	// 项目已删除：释放该项目的文件列表缓存
	service.InvalidateProjectFileList(projectNameDto.ProjectName)

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

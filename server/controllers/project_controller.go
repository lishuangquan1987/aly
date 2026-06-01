package controllers

import (
	"clientupdator/server/ent"
	"clientupdator/server/ent/project"
	"clientupdator/server/internal/db"
	"clientupdator/server/internal/service"
	"clientupdator/server/models"
	"fmt"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/utils-go/ngo/io/directory"
)

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

	//判断项目名称是否为空
	if createProjectDto.Name == "" {
		ctx.JSON(200, models.NG("项目名称不能为空"))
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
	result := service.CreateProjectWithFirstLog(
		createProjectDto.Name,
		createProjectDto.Title,
		createProjectDto.IsForceUpdate,
		createProjectDto.IgnoreFolders,
		createProjectDto.IgnoreFiles)

	ctx.JSON(200, result)
}

func UpdateProject(ctx *gin.Context) {
	var updateProjectDto struct {
		ID            int      `json:"id"`
		Title         string   `json:"title"`
		IsForceUpdate bool     `json:"isForceUpdate"`
		IgnoreFolders []string `json:"ignoreFolders"`
		IgnoreFiles   []string `json:"ignoreFiles"`
	}
	if err := ctx.ShouldBindJSON(&updateProjectDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if updateProjectDto.ID <= 0 {
		ctx.JSON(200, models.NG("项目ID不能为空"))
		return
	}

	//判断项目名称是否为空
	if updateProjectDto.Title == "" {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}

	//判断项目名称是否存在
	_, err := db.Client.Project.Query().Where(project.IDEQ(int(updateProjectDto.ID))).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			ctx.JSON(200, models.NG("项目不存在"))
		} else {
			ctx.JSON(200, models.NGWithError(err))
		}
		return
	}

	//更新
	result := service.UpdateProject(
		updateProjectDto.ID,
		updateProjectDto.Title,
		updateProjectDto.IsForceUpdate,
		updateProjectDto.IgnoreFolders,
		updateProjectDto.IgnoreFiles)

	ctx.JSON(200, result)
}

func PublishVersion(ctx *gin.Context) {
	var publishDto struct {
		ProjectId int      `json:"projectId"`
		Version   string   `json:"version"`
		Logs      []string `json:"logs"`
		Time      string   `json:"time"`
	}
	if err := ctx.ShouldBindJSON(&publishDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if publishDto.ProjectId <= 0 {
		ctx.JSON(200, models.NG("项目ID不能为空"))
		return
	}
	if publishDto.Version == "" {
		ctx.JSON(200, models.NG("版本号不能为空"))
		return
	}

	ctx.JSON(200, service.PublishVersion(publishDto.ProjectId, publishDto.Version, publishDto.Logs, publishDto.Time))
}

func DeleteProject(ctx *gin.Context) {
	var projectIdDto struct {
		ProjectId int `uri:"projectId" json:"projectId"`
	}
	if err := ctx.BindUri(&projectIdDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	ctx.JSON(200, service.DeleteProject(projectIdDto.ProjectId))
}

func GetAllProjects(ctx *gin.Context) {
	ctx.JSON(200, service.GetAllProjects())
}

func GetProjectChangeLogs(ctx *gin.Context) {
	var projectIdDto struct {
		ProjectId int `uri:"projectId" json:"projectId"`
	}
	if err := ctx.BindUri(&projectIdDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	if projectIdDto.ProjectId <= 0 {
		ctx.JSON(200, models.NG("项目ID不能为空"))
		return
	}

	ctx.JSON(200, service.GetProjectChangeLogs(projectIdDto.ProjectId))
}

func GetProjectOSInfo(ctx *gin.Context) {
	var projectIdUrl struct {
		ProjectId int `uri:"projectId" json:"projectId"`
	}
	if err := ctx.BindUri(&projectIdUrl); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	projectResult := service.GetProjectById(projectIdUrl.ProjectId)
	if !projectResult.IsSuccess {
		ctx.JSON(200, models.NG(projectResult.ErrorMsg))
		return
	}

	project := projectResult.Data.(*ent.Project)
	workDir, err := service.GetProjectWorkPath(project.Name)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	platform, _, _, _ := host.PlatformInformation() //内核信息

	infos, _ := cpu.Info() //cpu信息工具类

	diskInfo, _ := disk.Usage(workDir) //获取客户端更新文件所在盘的容量信息

	serverOSInfo := make([]models.ServerOSInfo, 0)
	serverOSInfo = append(serverOSInfo, models.ServerOSInfo{
		OS:              runtime.GOOS,
		Platform:        platform,
		GOARCH:          runtime.GOARCH,
		Version:         runtime.Version(),
		NumCPU:          runtime.NumCPU(),
		CPUName:         infos[0].ModelName,
		CPUMhz:          infos[0].Mhz,
		DiskUsed:        float64((diskInfo.Used) / (1024 * 1024 * 1024)), //Byte 转为操作系统的 Gib 单位
		DiskFree:        float64((diskInfo.Free) / (1024 * 1024 * 1024)),
		DiskTotal:       float64((diskInfo.Total) / (1024 * 1024 * 1024)),
		DiskUsedPercent: diskInfo.UsedPercent,
	})
	ctx.JSON(200, models.CommonResponse{
		IsSuccess: true,
		Data:      serverOSInfo,
	})
}



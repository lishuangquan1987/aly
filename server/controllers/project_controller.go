package controllers

import (
	"zap/server/ent"
	"zap/server/ent/project"
	"zap/server/internal/db"
	"zap/server/internal/service"
	"zap/server/models"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

	if !result.IsSuccess && strings.Contains(result.ErrorMsg, "UNIQUE") || strings.Contains(result.ErrorMsg, "unique") {
		ctx.JSON(200, models.NG(fmt.Sprintf("项目名称:%s已存在", createProjectDto.Name)))
		return
	}
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
	if updateProjectDto.Name == "" {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}

	//判断项目抬头是否为空
	if updateProjectDto.Title == "" {
		ctx.JSON(200, models.NG("项目抬头不能为空"))
		return
	}

	//判断项目是否存在
	_, err := db.Client.Project.Query().Where(project.NameEQ(updateProjectDto.Name)).First(ctx)
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
		updateProjectDto.Name,
		updateProjectDto.Title,
		updateProjectDto.IsForceUpdate,
		updateProjectDto.IgnoreFolders,
		updateProjectDto.IgnoreFiles)

	ctx.JSON(200, result)
}

func PublishVersion(ctx *gin.Context) {
	var publishDto struct {
		ProjectName string   `json:"projectName"`
		Version     string   `json:"version"`
		Logs        []string `json:"logs"`
		Time        string   `json:"time"`
	}
	if err := ctx.ShouldBindJSON(&publishDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if publishDto.ProjectName == "" {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}
	if publishDto.Version == "" {
		ctx.JSON(200, models.NG("版本号不能为空"))
		return
	}
	if len(publishDto.Version) > 50 {
		ctx.JSON(200, models.NG("版本号长度不能超过50个字符"))
		return
	}

	ctx.JSON(200, service.PublishVersion(publishDto.ProjectName, publishDto.Version, publishDto.Logs, publishDto.Time))
}

func DeleteProject(ctx *gin.Context) {
	var projectNameDto struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	ctx.JSON(200, service.DeleteProject(projectNameDto.ProjectName))
}

func GetAllProjects(ctx *gin.Context) {
	ctx.JSON(200, service.GetAllProjects())
}

func GetProjectChangeLogs(ctx *gin.Context) {
	var projectNameDto struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	if projectNameDto.ProjectName == "" {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}

	ctx.JSON(200, service.GetProjectChangeLogs(projectNameDto.ProjectName))
}

func GetProjectOSInfo(ctx *gin.Context) {
	var projectNameUrl struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameUrl); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	// 直接查询以便区分 "不存在" 和 "其他错误"
	p, err := db.Client.Project.Query().Where(project.NameEQ(projectNameUrl.ProjectName)).First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			ctx.JSON(200, models.NG("项目不存在"))
		} else {
			ctx.JSON(200, models.NGWithError(err))
		}
		return
	}

	workDir, err := service.GetProjectWorkPath(p.Name)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	platform, _, _, _ := host.PlatformInformation() //内核信息

	infos, err := cpu.Info() //cpu信息工具类
	var cpuName string
	var cpuMhz float64
	if err == nil && len(infos) > 0 {
		cpuName = infos[0].ModelName
		cpuMhz = infos[0].Mhz
	} else {
		cpuName = "unknown"
	}

	diskInfo, err := disk.Usage(workDir) //获取客户端更新文件所在盘的容量信息
	var diskUsed, diskFree, diskTotal uint64
	var diskUsedPercent float64
	if err == nil {
		diskUsed = diskInfo.Used
		diskFree = diskInfo.Free
		diskTotal = diskInfo.Total
		diskUsedPercent = diskInfo.UsedPercent
	}

	serverOSInfo := make([]models.ServerOSInfo, 0)
	serverOSInfo = append(serverOSInfo, models.ServerOSInfo{
		OS:              runtime.GOOS,
		Platform:        platform,
		GOARCH:          runtime.GOARCH,
		Version:         runtime.Version(),
		NumCPU:          runtime.NumCPU(),
		CPUName:         cpuName,
		CPUMhz:          cpuMhz,
		DiskUsed:        float64(diskUsed) / float64(1024*1024*1024), //Byte 转为操作系统的 Gib 单位
		DiskFree:        float64(diskFree) / float64(1024*1024*1024),
		DiskTotal:       float64(diskTotal) / float64(1024*1024*1024),
		DiskUsedPercent: diskUsedPercent,
	})
	ctx.JSON(200, models.CommonResponse{
		IsSuccess: true,
		Data:      serverOSInfo,
	})
}

// ServerInfo 获取服务器基本信息（无需指定项目）
func ServerInfo(ctx *gin.Context) {
	exePath, err := os.Executable()
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	workDir := filepath.Dir(exePath)

	platform, _, _, _ := host.PlatformInformation()

	infos, err := cpu.Info()
	var cpuName string
	var cpuMhz float64
	if err == nil && len(infos) > 0 {
		cpuName = infos[0].ModelName
		cpuMhz = infos[0].Mhz
	} else {
		cpuName = "unknown"
	}

	diskInfo, err := disk.Usage(workDir)
	var diskUsed, diskFree, diskTotal uint64
	var diskUsedPercent float64
	if err == nil {
		diskUsed = diskInfo.Used
		diskFree = diskInfo.Free
		diskTotal = diskInfo.Total
		diskUsedPercent = diskInfo.UsedPercent
	}

	serverOSInfo := make([]models.ServerOSInfo, 0)
	serverOSInfo = append(serverOSInfo, models.ServerOSInfo{
		OS:              runtime.GOOS,
		Platform:        platform,
		GOARCH:          runtime.GOARCH,
		Version:         runtime.Version(),
		NumCPU:          runtime.NumCPU(),
		CPUName:         cpuName,
		CPUMhz:          cpuMhz,
		DiskUsed:        float64(diskUsed) / float64(1024*1024*1024),
		DiskFree:        float64(diskFree) / float64(1024*1024*1024),
		DiskTotal:       float64(diskTotal) / float64(1024*1024*1024),
		DiskUsedPercent: diskUsedPercent,
	})
	ctx.JSON(200, models.CommonResponse{
		IsSuccess: true,
		Data:      serverOSInfo,
	})
}

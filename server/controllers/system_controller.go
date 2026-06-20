package controllers

import (
	"zap/server/ent"
	"zap/server/ent/project"
	"zap/server/internal/db"
	"zap/server/internal/service"
	"zap/server/models"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
)

// GetProjectOSInfo 获取项目所在服务器的系统信息
func GetProjectOSInfo(ctx *gin.Context) {
	var projectNameUrl struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameUrl); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	if err := validateProjectName(projectNameUrl.ProjectName); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return
	}

	// 直接查询以便区分 "不存在" 和 "其他错误"（仅未软删除的项目）
	p, err := db.Client.Project.Query().Where(project.NameEQ(projectNameUrl.ProjectName), project.IsDeletedEQ(false)).First(ctx)
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

	ctx.JSON(200, models.CommonResponse{
		IsSuccess: true,
		Data:      collectOSInfo(workDir),
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

	ctx.JSON(200, models.CommonResponse{
		IsSuccess: true,
		Data:      collectOSInfo(workDir),
	})
}

// collectOSInfo 收集操作系统信息（CPU、磁盘等），供 GetProjectOSInfo 和 ServerInfo 共用
func collectOSInfo(workDir string) []models.ServerOSInfo {
	platform, _, _, err := host.PlatformInformation()
	if err != nil {
		log.Printf("collectOSInfo: failed to get platform information: %v", err)
	}

	infos, err := cpu.Info()
	var cpuName string
	var cpuMhz float64
	if err == nil && len(infos) > 0 {
		cpuName = infos[0].ModelName
		cpuMhz = infos[0].Mhz
	} else {
		cpuName = "unknown"
		if err != nil {
			log.Printf("collectOSInfo: failed to get CPU info: %v", err)
		}
	}

	diskInfo, err := disk.Usage(workDir)
	var diskUsed, diskFree, diskTotal uint64
	var diskUsedPercent float64
	if err == nil {
		diskUsed = diskInfo.Used
		diskFree = diskInfo.Free
		diskTotal = diskInfo.Total
		diskUsedPercent = diskInfo.UsedPercent
	} else {
		log.Printf("collectOSInfo: failed to get disk usage for %s: %v", workDir, err)
	}

	return []models.ServerOSInfo{{
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
	}}
}

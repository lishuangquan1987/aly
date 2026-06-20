package controllers

import (
	"zap/server/internal/service"
	"zap/server/models"

	"github.com/gin-gonic/gin"
)

// PublishVersion 发布新版本
func PublishVersion(ctx *gin.Context) {
	var publishDto struct {
		ProjectName            string   `json:"projectName"`
		Version                string   `json:"version"`
		Logs                   []string `json:"logs"`
		Time                   string   `json:"time"`
		AfterApplyUpdateScript string   `json:"afterApplyUpdateScript"`
	}
	if err := ctx.ShouldBindJSON(&publishDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if err := validateProjectName(publishDto.ProjectName); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
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

	ctx.JSON(200, service.PublishVersion(ctx.Request.Context(), publishDto.ProjectName, publishDto.Version, publishDto.Logs, publishDto.Time, publishDto.AfterApplyUpdateScript))
}

// GetProjectChangeLogs 获取项目变更日志
func GetProjectChangeLogs(ctx *gin.Context) {
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

	ctx.JSON(200, service.GetProjectChangeLogs(ctx.Request.Context(), projectNameDto.ProjectName))
}

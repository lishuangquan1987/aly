package controllers

import (
	"fmt"
	"regexp"

	"aly/server/internal/service"
	"aly/server/models"

	"github.com/gin-gonic/gin"
)

// validVersion 版本号格式校验（#12）：数字段（至少 1 段，如 "2" 也合法——
// 与客户端 isLikelyVersion 口径对齐）+ 可选预发布/构建元数据后缀。
// 兼容 v 前缀由服务端 CompareVersion 剥离，正则统一允许。
var validVersion = regexp.MustCompile(`^v?\d+(\.\d+)*(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

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
	// #12 版本格式口径统一：格式明显非法（如 "2026.10.31 15:00" 带空格、
	// "1.0.0 beta" 等）直接拒绝，防止整条发布链路各自按不同口径解析。
	if !validVersion.MatchString(publishDto.Version) {
		ctx.JSON(200, models.NG(fmt.Sprintf("版本号格式非法: %s（应为数字版本号，如 1.0.0 / 2.1.0-beta / v1.2.3）", publishDto.Version)))
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

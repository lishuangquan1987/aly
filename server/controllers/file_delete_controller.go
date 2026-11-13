package controllers

import (
	"os"
	"strings"

	"aly/server/internal/service"
	"aly/server/models"

	"github.com/gin-gonic/gin"
)

// DeleteFile 删除项目内的一个文件（审查 B6：删除同步链路）。
//
// 背景：publish-cli 的 diff 一直能识别 "deleted" 状态，但全链路没有删除 API，
// 本地删掉的文件会永久残留、客户端更新后旧文件继续留在目标目录。
//
// 语义：幂等删除——文件不存在也返回成功（发布端可能重复重试）。
// 服务端文件清单来自磁盘遍历，删除后使缓存失效即可，客户端靠
// .updator_manifest.json 差量自行删除本地旧文件。
func DeleteFile(ctx *gin.Context) {
	var req struct {
		ProjectName      string `json:"projectName" form:"projectName"`
		RelativeFileName string `json:"relativeFileName" form:"relativeFileName"`
	}
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if len(req.ProjectName) == 0 {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}
	if len(req.RelativeFileName) == 0 {
		ctx.JSON(200, models.NG("文件路径不能为空"))
		return
	}

	// 项目必须存在且未软删除（与上传同口径）
	if !checkUploadProject(ctx, req.ProjectName) {
		return
	}

	// 项目名校验 + 落点断言：确保只在项目目录内删除
	workDir, absFileName, ok := resolveUploadTarget(ctx, req.ProjectName, req.RelativeFileName)
	if !ok {
		return
	}
	// resolveUploadTarget 只允许"workDir 之下的文件"，但删除时还要额外拒绝
	// workDir 本身与项目根目录
	if absFileName == workDir || strings.EqualFold(absFileName, workDir) {
		ctx.JSON(200, models.NG("非法的文件路径"))
		return
	}

	// 幂等：不存在视为已删除成功
	if err := os.Remove(absFileName); err != nil && !os.IsNotExist(err) {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	// 顺手清理该文件可能残留的分片目录
	_ = os.RemoveAll(absFileName + ".chunks")

	// 文件已变更：使该项目的文件列表缓存失效
	service.InvalidateProjectFileList(req.ProjectName)

	ctx.JSON(200, models.OK())
}

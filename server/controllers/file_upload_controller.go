package controllers

import (
	"aly/server/internal/service"
	"aly/server/models"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/directory"
	"github.com/utils-go/ngo/io/path"
	"github.com/utils-go/ngo/stringUtils"
)

func UploadFile(ctx *gin.Context) {
	f, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	var fileInfo struct {
		ProjectName string `form:"projectName"`
		RelativeFileName    string `form:"relativeFileName"`
	}
	if err := ctx.ShouldBind(&fileInfo); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	if len(fileInfo.ProjectName) == 0 {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}
	if len(fileInfo.RelativeFileName) == 0 {
		ctx.JSON(200, models.NG("header中必须包含FileName"))
		return
	}

	workDir, err := service.GetProjectWorkPath(fileInfo.ProjectName)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	// 如果项目文件夹不存在则自动创建
	if !directory.Exists(workDir) {
		if err := directory.CreateDirectory(workDir); err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
	}

	fileName := stringUtils.Replace(fileInfo.RelativeFileName, "\\", "/")
	// 路径穿越防护：规范化路径并验证在 workDir 内
	rawPath := path.Combine(workDir, fileName)
	absFileName := filepath.Clean(rawPath)
	if !strings.HasPrefix(absFileName, workDir+string(filepath.Separator)) && absFileName != workDir {
		ctx.JSON(200, models.NG("非法的文件路径"))
		return
	}
	dir := path.GetDirectoryName(absFileName)
	if !directory.Exists(dir) {
		if err = directory.CreateDirectory(dir); err != nil {
			ctx.JSON(200, models.NG(fmt.Sprintf("create directory error:%v", err)))
			return
		}
	}

	if err = ctx.SaveUploadedFile(f, absFileName); err != nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("save upload file error:%v", err)))
		return
	}

	ctx.JSON(200, models.OK())
}

// UploadChunk 接收分片上传：将单个分片保存到临时目录
func UploadChunk(ctx *gin.Context) {
	var chunkInfo struct {
		ProjectName      string `form:"projectName"`
		RelativeFileName string `form:"relativeFileName"`
		ChunkIndex       int    `form:"chunkIndex"`
		TotalChunks      int    `form:"totalChunks"`
	}
	if err := ctx.ShouldBind(&chunkInfo); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if len(chunkInfo.ProjectName) == 0 {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}
	if len(chunkInfo.RelativeFileName) == 0 {
		ctx.JSON(200, models.NG("文件路径不能为空"))
		return
	}
	if chunkInfo.ChunkIndex < 0 || chunkInfo.ChunkIndex >= chunkInfo.TotalChunks {
		ctx.JSON(200, models.NG(fmt.Sprintf("分片序号不合法: chunkIndex=%d, totalChunks=%d", chunkInfo.ChunkIndex, chunkInfo.TotalChunks)))
		return
	}

	f, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	workDir, err := service.GetProjectWorkPath(chunkInfo.ProjectName)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if !directory.Exists(workDir) {
		if err := directory.CreateDirectory(workDir); err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
	}

	fileName := stringUtils.Replace(chunkInfo.RelativeFileName, "\\", "/")
	rawPath := path.Combine(workDir, fileName)
	absFileName := filepath.Clean(rawPath)
	if !strings.HasPrefix(absFileName, workDir+string(filepath.Separator)) && absFileName != workDir {
		ctx.JSON(200, models.NG("非法的文件路径"))
		return
	}

	// 分片暂存目录：{文件路径}.chunks/
	chunksDir := absFileName + ".chunks"
	if !directory.Exists(chunksDir) {
		if err := directory.CreateDirectory(chunksDir); err != nil {
			ctx.JSON(200, models.NG(fmt.Sprintf("create chunks dir error: %v", err)))
			return
		}
	}

	chunkPath := filepath.Join(chunksDir, strconv.Itoa(chunkInfo.ChunkIndex))
	if err := ctx.SaveUploadedFile(f, chunkPath); err != nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("save chunk error: %v", err)))
		return
	}

	ctx.JSON(200, models.OK())
}

// UploadChunkComplete 合并所有分片为最终文件并清理临时目录
func UploadChunkComplete(ctx *gin.Context) {
	var info struct {
		ProjectName      string `form:"projectName"`
		RelativeFileName string `form:"relativeFileName"`
		TotalChunks      int    `form:"totalChunks"`
	}
	if err := ctx.ShouldBind(&info); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if len(info.ProjectName) == 0 {
		ctx.JSON(200, models.NG("项目名称不能为空"))
		return
	}
	if len(info.RelativeFileName) == 0 {
		ctx.JSON(200, models.NG("文件路径不能为空"))
		return
	}

	workDir, err := service.GetProjectWorkPath(info.ProjectName)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	fileName := stringUtils.Replace(info.RelativeFileName, "\\", "/")
	rawPath := path.Combine(workDir, fileName)
	absFileName := filepath.Clean(rawPath)
	if !strings.HasPrefix(absFileName, workDir+string(filepath.Separator)) && absFileName != workDir {
		ctx.JSON(200, models.NG("非法的文件路径"))
		return
	}

	chunksDir := absFileName + ".chunks"
	if !directory.Exists(chunksDir) {
		ctx.JSON(200, models.NG("未找到分片目录，请先上传分片"))
		return
	}

	// 验证所有分片存在、文件大小
	var missing []int
	var totalSize int64
	for i := 0; i < info.TotalChunks; i++ {
		chunkPath := filepath.Join(chunksDir, strconv.Itoa(i))
		fi, err := os.Stat(chunkPath)
		if err != nil {
			missing = append(missing, i)
		} else {
			totalSize += fi.Size()
		}
	}
	if len(missing) > 0 {
		ctx.JSON(200, models.OKWithData(map[string]interface{}{
			"chunksComplete": false,
			"missingChunks":  missing,
		}))
		return
	}

	// 确保目标目录存在
	destDir := path.GetDirectoryName(absFileName)
	if !directory.Exists(destDir) {
		if err := directory.CreateDirectory(destDir); err != nil {
			ctx.JSON(200, models.NG(fmt.Sprintf("create dest dir error: %v", err)))
			return
		}
	}

	// 合并分片到最终文件
	destFile, err := os.Create(absFileName)
	if err != nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("create dest file error: %v", err)))
		return
	}
	defer destFile.Close()

	for i := 0; i < info.TotalChunks; i++ {
		chunkPath := filepath.Join(chunksDir, strconv.Itoa(i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			ctx.JSON(200, models.NG(fmt.Sprintf("open chunk %d error: %v", i, err)))
			return
		}
		if _, err := io.Copy(destFile, chunkFile); err != nil {
			chunkFile.Close()
			ctx.JSON(200, models.NG(fmt.Sprintf("copy chunk %d error: %v", i, err)))
			return
		}
		chunkFile.Close()
	}

	// 清理分片目录
	os.RemoveAll(chunksDir)

	ctx.JSON(200, models.OKWithData(map[string]interface{}{
		"chunksComplete": true,
		"totalSize":      totalSize,
	}))
}

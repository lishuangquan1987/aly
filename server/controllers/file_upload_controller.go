package controllers

import (
	"aly/server/internal/service"
	"aly/server/models"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/directory"
	"github.com/utils-go/ngo/io/path"
	"github.com/utils-go/ngo/stringUtils"
)

// maxUploadFileSize 单文件上传大小上限（1GB），防止恶意/异常上传填满磁盘
const maxUploadFileSize int64 = 1 << 30

func UploadFile(ctx *gin.Context) {
	// 上传大小前置限制（#14）：
	// 1) Content-Length 早退，不解析 body；
	// 2) MaxBytesReader 在 multipart 解析阶段即截断超限 body，避免大文件先落盘再拒绝。
	if ctx.Request.ContentLength > maxUploadFileSize {
		ctx.JSON(200, models.NG(fmt.Sprintf("文件过大: %d 字节，上限 %d 字节", ctx.Request.ContentLength, maxUploadFileSize)))
		return
	}
	// 预留 1MB 覆盖 multipart 边界/表单字段开销
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadFileSize+1<<20)

	f, err := ctx.FormFile("file")
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ctx.JSON(200, models.NG(fmt.Sprintf("文件过大: 上限 %d 字节", maxUploadFileSize)))
			return
		}
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	// 兜底：multipart 解析后仍校验大小
	if f.Size > maxUploadFileSize {
		ctx.JSON(200, models.NG(fmt.Sprintf("文件过大: %d 字节，上限 %d 字节", f.Size, maxUploadFileSize)))
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

// chunkMaxSize 单分片大小上限（64MB）：publish-cli 实际按 1MB 分包，64MB 是宽松上限，
// 防止单请求写入近 1GB 临时文件（#14 审查发现）。
const chunkMaxSize int64 = 64 << 20

// UploadChunk 接收分片上传：将单个分片保存到临时目录
func UploadChunk(ctx *gin.Context) {
	// 上传大小前置限制（#14）：必须在 ShouldBind 解析 multipart body 之前安装，
	// 否则 body 已被完整解析落盘，限制形同虚设。
	if ctx.Request.ContentLength > chunkMaxSize {
		ctx.JSON(200, models.NG(fmt.Sprintf("分片过大: %d 字节，上限 %d 字节", ctx.Request.ContentLength, chunkMaxSize)))
		return
	}
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, chunkMaxSize+1<<20)

	var chunkInfo struct {
		ProjectName      string `form:"projectName"`
		RelativeFileName string `form:"relativeFileName"`
		ChunkIndex       int    `form:"chunkIndex"`
		TotalChunks      int    `form:"totalChunks"`
	}
	if err := ctx.ShouldBind(&chunkInfo); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			ctx.JSON(200, models.NG(fmt.Sprintf("分片过大: 上限 %d 字节", chunkMaxSize)))
			return
		}
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
	// 分片大小校验（解析后兜底）
	if f.Size > chunkMaxSize {
		ctx.JSON(200, models.NG(fmt.Sprintf("分片过大: %d 字节，上限 %d 字节", f.Size, chunkMaxSize)))
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
	// 累计大小校验
	if totalSize > maxUploadFileSize {
		ctx.JSON(200, models.NG(fmt.Sprintf("文件过大: %d 字节，上限 %d 字节", totalSize, maxUploadFileSize)))
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

	// 合并分片到临时文件，成功后原子 rename。
	// 旧实现先 os.Create 截断目标：复制中途失败留下半截文件，并发下载会读到损坏数据。
	mergePath := absFileName + ".merging"
	destFile, err := os.Create(mergePath)
	if err != nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("create dest file error: %v", err)))
		return
	}

	for i := 0; i < info.TotalChunks; i++ {
		chunkPath := filepath.Join(chunksDir, strconv.Itoa(i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			destFile.Close()
			os.Remove(mergePath)
			ctx.JSON(200, models.NG(fmt.Sprintf("open chunk %d error: %v", i, err)))
			return
		}
		if _, err := io.Copy(destFile, chunkFile); err != nil {
			chunkFile.Close()
			destFile.Close()
			os.Remove(mergePath)
			ctx.JSON(200, models.NG(fmt.Sprintf("copy chunk %d error: %v", i, err)))
			return
		}
		chunkFile.Close()
	}
	closeErr := destFile.Close()
	if closeErr != nil {
		os.Remove(mergePath)
		ctx.JSON(200, models.NG(fmt.Sprintf("close merged file error: %v", closeErr)))
		return
	}
	if err := os.Rename(mergePath, absFileName); err != nil {
		os.Remove(mergePath)
		ctx.JSON(200, models.NG(fmt.Sprintf("atomic rename merged file error: %v", err)))
		return
	}

	// 清理分片目录
	os.RemoveAll(chunksDir)

	ctx.JSON(200, models.OKWithData(map[string]interface{}{
		"chunksComplete": true,
		"totalSize":      totalSize,
	}))
}

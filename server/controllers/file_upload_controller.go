package controllers

import (
	"crypto/md5"
	"encoding/hex"
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

	// 审查 M5：上传前要求项目已存在于库中（未软删除）——否则任何人可借上传
	// 在服务端任意位置建目录、写文件。
	if !checkUploadProject(ctx, fileInfo.ProjectName) {
		return
	}

	// 审查 B1：项目名校验 + 落点断言（GetProjectWorkPath 内含 data 目录断言）
	workDir, absFileName, ok := resolveUploadTarget(ctx, fileInfo.ProjectName, fileInfo.RelativeFileName)
	if !ok {
		return
	}

	// 如果项目文件夹不存在则自动创建
	if !directory.Exists(workDir) {
		if err := directory.CreateDirectory(workDir); err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
	}

	// 审查 M5：磁盘空间检查（含 512MB 预留）
	if !checkDiskSpace(ctx, workDir, f.Size) {
		return
	}

	dir := path.GetDirectoryName(absFileName)
	if !directory.Exists(dir) {
		if err = directory.CreateDirectory(dir); err != nil {
			ctx.JSON(200, models.NG(fmt.Sprintf("create directory error:%v", err)))
			return
		}
	}
	// 原子落盘（#3）：先保存到 .merging 临时文件，再 rename 切换 ——
	// 与分片上传路径（UploadChunkComplete）同款保护。旧实现直接写最终路径，
	// 上传进行中并发 get_all_files 会把半截文件当正式文件立刻缓存下发（撕裂读）。
	// .merging 属上传中间态，file_cache 的 IsUploadTempPath 会跳过。
	mergePath := absFileName + ".merging"
	if err = ctx.SaveUploadedFile(f, mergePath); err != nil {
		ctx.JSON(200, models.NG(fmt.Sprintf("save upload file error:%v", err)))
		return
	}
	if err = os.Rename(mergePath, absFileName); err != nil {
		os.Remove(mergePath)
		ctx.JSON(200, models.NG(fmt.Sprintf("atomic rename upload file error:%v", err)))
		return
	}

	// 文件已变更：使该项目的文件列表缓存失效，下次 get_all_files 重新缓存
	service.InvalidateProjectFileList(fileInfo.ProjectName)

	ctx.JSON(200, models.OK())
}

// chunkMaxSize 单分片大小上限（64MB）：publish-cli 默认按 128KiB 分包
// （defaultChunkSize=128*1024，可用 ALY_CHUNK_SIZE 覆盖），64MB 是宽松上限，
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
	// 审查 N1：totalChunks 上界校验（与 complete 同一防护点）
	if chunkInfo.TotalChunks > maxChunkTotalChunks {
		ctx.JSON(200, models.NG(fmt.Sprintf("分片总数不合法: totalChunks=%d, 上限 %d", chunkInfo.TotalChunks, maxChunkTotalChunks)))
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

	// 审查 M5：项目必须存在且未软删除
	if !checkUploadProject(ctx, chunkInfo.ProjectName) {
		return
	}

	// 审查 B1：项目名校验 + 落点断言
	workDir, absFileName, ok := resolveUploadTarget(ctx, chunkInfo.ProjectName, chunkInfo.RelativeFileName)
	if !ok {
		return
	}
	if !directory.Exists(workDir) {
		if err := directory.CreateDirectory(workDir); err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
	}
	// 审查 M5：磁盘空间检查（分片累加未知，按单分片 + 预留估计）
	if !checkDiskSpace(ctx, workDir, f.Size) {
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

	// 分片上传也是 upload 调用：按用户要求使缓存失效（合并完成后会再次失效）
	service.InvalidateProjectFileList(chunkInfo.ProjectName)

	ctx.JSON(200, models.OK())
}

// UploadChunkComplete 合并所有分片为最终文件并清理临时目录
func UploadChunkComplete(ctx *gin.Context) {
	var info struct {
		ProjectName      string `form:"projectName"`
		RelativeFileName string `form:"relativeFileName"`
		TotalChunks      int    `form:"totalChunks"`
		// Md5 期望的整文件 MD5（审查 M3）：可选，服务端合并后计算比对，
		// 从根上消灭"合并结果与发布者意图不一致却静默下发"的一类问题。
		Md5 string `form:"md5"`
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
	// 审查 B3 + N1：totalChunks 必须为正且在合理上界内。
	// 旧实现 `for i := 0; i < info.TotalChunks; i++` 遇 0/负数不执行循环 →
	// missing 为空、totalSize=0 不超限 → 创建空 .merging 后原子 rename
	// **覆盖目标文件为 0 字节**；上界缺失还可被单请求打成 OOM/CPU 自旋。
	if info.TotalChunks <= 0 || info.TotalChunks > maxChunkTotalChunks {
		ctx.JSON(200, models.NG(fmt.Sprintf("totalChunks 不合法: %d（允许范围 1-%d）", info.TotalChunks, maxChunkTotalChunks)))
		return
	}

	// 审查 M5：项目必须存在且未软删除
	if !checkUploadProject(ctx, info.ProjectName) {
		return
	}

	// 审查 B1：项目名校验 + 落点断言
	workDir, absFileName, ok := resolveUploadTarget(ctx, info.ProjectName, info.RelativeFileName)
	if !ok {
		return
	}

	chunksDir := absFileName + ".chunks"
	if !directory.Exists(chunksDir) {
		ctx.JSON(200, models.NG("未找到分片目录，请先上传分片"))
		return
	}

	// 审查 M3：按目标文件加锁，避免两个发布者并发 complete 同一文件时
	// 互相清掉对方分片、静默产出损坏文件。
	fileLock := lockForFile(absFileName)
	fileLock.Lock()
	defer fileLock.Unlock()

	// 验证所有分片存在、累计文件大小
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
	// 审查 B3：分片都在但全部为空同样会造出 0 字节覆盖，必须拒绝
	if totalSize == 0 {
		ctx.JSON(200, models.NG("分片内容为空，拒绝合并（避免把目标文件清空）"))
		return
	}
	// 累计大小校验
	if totalSize > maxUploadFileSize {
		ctx.JSON(200, models.NG(fmt.Sprintf("文件过大: %d 字节，上限 %d 字节", totalSize, maxUploadFileSize)))
		return
	}
	// 审查 M5：磁盘空间检查
	if !checkDiskSpace(ctx, workDir, totalSize) {
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

	// 审查 M3：边合并边算 MD5，与发布者给出的期望值比对
	hasher := md5.New()
	writer := io.MultiWriter(destFile, hasher)

	for i := 0; i < info.TotalChunks; i++ {
		chunkPath := filepath.Join(chunksDir, strconv.Itoa(i))
		chunkFile, err := os.Open(chunkPath)
		if err != nil {
			destFile.Close()
			os.Remove(mergePath)
			ctx.JSON(200, models.NG(fmt.Sprintf("open chunk %d error: %v", i, err)))
			return
		}
		if _, err := io.Copy(writer, chunkFile); err != nil {
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

	// 审查 M3：期望 MD5 校验（未提供时保持向后兼容，跳过）
	actualMd5 := hex.EncodeToString(hasher.Sum(nil))
	if info.Md5 != "" && !strings.EqualFold(info.Md5, actualMd5) {
		// 保留 .chunks 供发布者在修正后重试，不删除现场
		os.Remove(mergePath)
		ctx.JSON(200, models.NG(fmt.Sprintf("合并结果 MD5 不符: 期望 %s，实际 %s（分片已保留，可重试）", info.Md5, actualMd5)))
		return
	}

	if err := os.Rename(mergePath, absFileName); err != nil {
		os.Remove(mergePath)
		ctx.JSON(200, models.NG(fmt.Sprintf("atomic rename merged file error: %v", err)))
		return
	}

	// 清理分片目录
	os.RemoveAll(chunksDir)
	// 文件已变更：使该项目的文件列表缓存失效，下次 get_all_files 重新缓存
	service.InvalidateProjectFileList(info.ProjectName)

	ctx.JSON(200, models.OKWithData(map[string]interface{}{
		"chunksComplete": true,
		"totalSize":      totalSize,
		"md5":            actualMd5,
	}))
}

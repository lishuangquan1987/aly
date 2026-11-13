package controllers

import (
	"aly/server/ent"
	"aly/server/internal/pathcfg"
	"aly/server/internal/service"
	"aly/server/models"
	"encoding/json"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/path"
)

func GetAllFilesByProjectName(ctx *gin.Context) {
	var projectNameDto struct {
		ProjectName string `uri:"projectName" json:"projectName"`
	}
	if err := ctx.BindUri(&projectNameDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	projectResult := service.GetProjectByName(ctx.Request.Context(), projectNameDto.ProjectName)
	if !projectResult.IsSuccess {
		ctx.JSON(200, projectResult)
		return
	}

	p := projectResult.Data.(*ent.Project)

	workDir, err := service.GetProjectWorkPath(p.Name)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	// 按项目缓存文件列表（含 md5/sha256）：upload 时失效，下次请求重新缓存，
	// 避免每次 get_all_files 都对项目目录全量重算哈希（#13 性能问题）。
	fileInfos, err := service.GetProjectFileList(p.Name, workDir, p.IgnoreFolders, p.IgnoreFiles)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	ctx.JSON(200, models.OKWithData(fileInfos))
}

// GetReleaseManifest 返回项目当前发布清单（#4 删除同步）。
// 清单由 PublishVersion 成功时写入 data/{project}/.updator_manifest.json；
// 项目从未发布过（无清单）时返回空清单（客户端跳过删除，兼容老服务器/老项目）。
func GetReleaseManifest(ctx *gin.Context) {
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

	// 审查 M2/N2：清单接口旧实现完全不查库——软删除项目（甚至从未创建的项目）
	// 只要磁盘上有清单文件就照常返回。补项目存在且未删除校验。
	if projectResult := service.GetProjectByName(ctx.Request.Context(), projectNameDto.ProjectName); !projectResult.IsSuccess {
		ctx.JSON(200, projectResult)
		return
	}

	workDir, err := service.GetProjectWorkPath(projectNameDto.ProjectName)
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	manifestPath := filepath.Join(workDir, service.ManifestFileName)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 从未发布过：空清单（客户端据此跳过删除）
			ctx.JSON(200, models.OKWithData(struct {
				Files []string `json:"files"`
			}{Files: []string{}}))
			return
		}
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	var manifest struct {
		Files []string `json:"files"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}
	if manifest.Files == nil {
		manifest.Files = []string{}
	}

	ctx.JSON(200, models.OKWithData(manifest))
}

func DownloadFile(ctx *gin.Context) {
	pathStr := ctx.Query("path")

	// 路径穿越防护：规范化路径并限制在 data 目录内
	cleanPath := filepath.Clean(pathStr)
	// 拒绝下载上传中间态文件（#7 纵深防御）
	if service.IsUploadTempPath(filepath.ToSlash(cleanPath)) {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	// 审查 A7：data 目录基准与其它模块同源（pathcfg），避免"校验一套、落位另一套"
	dataDir, err := pathcfg.DataDir()
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(cleanPath, dataDir+string(filepath.Separator)) && cleanPath != dataDir {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	// 使用 http.ServeContent 支持 Range 请求（断点续传）
	// 不预先检查文件是否存在，直接打开，避免 TOCTOU 竞态
	fileReader, err := os.Open(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.AbortWithStatus(http.StatusNotFound)
		} else {
			ctx.AbortWithStatus(http.StatusInternalServerError)
		}
		return
	}
	defer fileReader.Close()

	fileInfo, err := fileReader.Stat()
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// 审查 B11：全局 WriteTimeout 已置 0（大文件下载不能有固定总时限），
	// 这里按文件大小给本次响应设置写期限，避免慢速读客户端长期占用连接。
	writeBudget := downloadWriteBudget(fileInfo.Size())
	if rc := http.NewResponseController(ctx.Writer); rc != nil {
		if err := rc.SetWriteDeadline(time.Now().Add(writeBudget)); err != nil {
			// 不支持 SetWriteDeadline 的 ResponseWriter 不阻断下载
			log.Printf("DownloadFile: SetWriteDeadline unsupported (non-fatal): %v", err)
		}
	}

	fileName := path.GetFileName(cleanPath)
	// 审查 M1：改用 mime.FormatMediaType 生成符合 RFC 6266/5987 的
	// Content-Disposition（非 ASCII 文件名走 filename* 的百分号编码，
	// 引号/换行等特殊字符由此函数转义）。非法输入时退回旧拼接兜底。
	disposition := ""
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": fileName}); v != "" {
		disposition = v
	} else {
		encodedName := strings.Replace(fileName, " ", "%20", -1)
		disposition = "attachment; filename*=UTF-8''" + encodedName + "; filename=" + fileName
	}
	ctx.Header("Content-Disposition", disposition)
	http.ServeContent(ctx.Writer, ctx.Request, fileName, fileInfo.ModTime(), fileReader)
}

// downloadWriteBudget 按文件大小估算下载写期限（审查 B11）：
// 以 1MB/s 的保守带宽估算，再加 5 分钟固定余量，上限 30 分钟。
func downloadWriteBudget(size int64) time.Duration {
	const minBudget = 5 * time.Minute
	const maxBudget = 30 * time.Minute
	budget := minBudget + time.Duration(size/(1<<20))*time.Second
	if budget > maxBudget {
		return maxBudget
	}
	return budget
}

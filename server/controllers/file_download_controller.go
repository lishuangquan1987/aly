package controllers

import (
	"aly/server/ent"
	"aly/server/internal/service"
	"aly/server/models"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	exePath, err := os.Executable()
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	dataDir := filepath.Join(filepath.Dir(exePath), service.DataDirName)
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

	fileName := path.GetFileName(cleanPath)
	// RFC 6266: filename*=UTF-8''<percent-encoded> 支持非 ASCII 文件名
	encodedName := strings.ReplaceAll(fileName, " ", "%20")
	ctx.Header("Content-Disposition", "attachment; filename*=UTF-8''"+encodedName+"; filename="+fileName)
	ctx.Header("Content-Transfer-Encoding", "binary")
	http.ServeContent(ctx.Writer, ctx.Request, fileName, fileInfo.ModTime(), fileReader)
}

package controllers

import (
	"clientupdator/server/ent"
	"clientupdator/server/internal/service"
	"clientupdator/server/internal/utils"
	"clientupdator/server/models"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/file"
	"github.com/utils-go/ngo/io/fileinfo"
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

	projectResult := service.GetProjectByName(projectNameDto.ProjectName)
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

	var fileInfos []models.FileInfo
	err = filepath.Walk(workDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 获取相对路径（正斜杠统一）
		relPath, _ := filepath.Rel(workDir, absPath)
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// 应用忽略文件夹规则
		for _, ignoreFolder := range p.IgnoreFolders {
			if strings.HasPrefix(relPath, ignoreFolder+"/") || relPath == ignoreFolder {
				return nil
			}
		}
		// 应用忽略文件规则
		for _, ignoreFile := range p.IgnoreFiles {
			if ignoreFile == relPath {
				return nil
			}
		}

		md5Str, err := utils.GetFileMD5(absPath)
		if err != nil {
			return err
		}
		sha256Str, err := utils.GetFileSHA256(absPath)
		if err != nil {
			return err
		}
		finfo := fileinfo.GetFileInfo(absPath)
		fileInfos = append(fileInfos, models.FileInfo{
			FileAbsolutePath: absPath,
			FileRelativePath: relPath,
			LastUpdateTime:   finfo.LastWriteTime,
			FileSize:         finfo.Length,
			MD5:              md5Str,
			SHA256:           sha256Str,
		})
		return nil
	})
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	ctx.JSON(200, models.OKWithData(fileInfos))
}

func DownloadFile(ctx *gin.Context) {
	pathStr := ctx.Query("path")

	// 路径穿越防护：规范化路径并限制在 data 目录内
	cleanPath := filepath.Clean(pathStr)
	exePath, err := os.Executable()
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	dataDir := filepath.Join(filepath.Dir(exePath), "data")
	if !strings.HasPrefix(cleanPath, dataDir+string(filepath.Separator)) && cleanPath != dataDir {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	if !file.Exists(cleanPath) {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	// 使用 http.ServeContent 支持 Range 请求（断点续传）
	fileReader, err := os.Open(cleanPath)
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}
	defer fileReader.Close()

	fileInfo, err := fileReader.Stat()
	if err != nil {
		ctx.AbortWithStatus(http.StatusNotFound)
		return
	}

	fileName := path.GetFileName(cleanPath)
	// RFC 6266: filename*=UTF-8''<percent-encoded> 支持非 ASCII 文件名
	encodedName := strings.ReplaceAll(fileName, " ", "%20")
	ctx.Header("Content-Disposition", "attachment; filename*=UTF-8''"+encodedName+"; filename="+fileName)
	ctx.Header("Content-Transfer-Encoding", "binary")
	http.ServeContent(ctx.Writer, ctx.Request, fileName, fileInfo.ModTime(), fileReader)
}

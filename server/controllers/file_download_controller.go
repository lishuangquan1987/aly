package controllers

import (
	"aly/server/ent"
	"aly/server/internal/service"
	"aly/server/internal/utils"
	"aly/server/models"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/io/directory"
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

	var fileInfos []models.FileInfo
	// 如果项目文件夹不存在（如项目刚创建还未上传文件），返回空列表
	if !directory.Exists(workDir) {
		ctx.JSON(200, models.OKWithData(fileInfos))
		return
	}
	err = filepath.Walk(workDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		// 获取相对路径（正斜杠统一）
		relPath, err := filepath.Rel(workDir, absPath)
		if err != nil {
			return err
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// 应用忽略文件夹规则
		for _, ignoreFolder := range p.IgnoreFolders {
			if strings.HasPrefix(relPath, ignoreFolder+"/") || relPath == ignoreFolder {
				return nil
			}
		}
		// 应用忽略文件规则（支持 glob 匹配）
		for _, ignoreFile := range p.IgnoreFiles {
			if matchIgnoreFile(relPath, ignoreFile) {
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

	// 按相对路径排序，确保每次返回顺序一致
	sort.Slice(fileInfos, func(i, j int) bool {
		return fileInfos[i].FileRelativePath < fileInfos[j].FileRelativePath
	})

	ctx.JSON(200, models.OKWithData(fileInfos))
}

// matchIgnoreFile 判断文件路径是否匹配忽略规则
// 与 publish-cli scanner.go 的 matchFile 保持一致的逻辑：
// 精确匹配 → glob 匹配（全路径）→ glob 匹配（文件名）→ *.ext 后缀匹配
// 注意：pattern 为 "*" 时，suffix 为空，strings.HasSuffix 恒返回 true，
// 即忽略所有文件（类似 .gitignore 的 * 规则），这是有意设计。
func matchIgnoreFile(relPath, pattern string) bool {
	if relPath == pattern {
		return true
	}
	if matched, err := filepath.Match(pattern, relPath); matched {
		return true
	} else if err != nil {
		log.Printf("WARN: invalid ignore pattern %q: %v", pattern, err)
	}
	base := filepath.Base(relPath)
	if matched, err := filepath.Match(pattern, base); matched {
		return true
	} else if err != nil {
		log.Printf("WARN: invalid ignore pattern %q: %v", pattern, err)
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(relPath, pattern[1:])
	}
	return false
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

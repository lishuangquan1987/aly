package controllers

import (
	"clientupdator/server/ent"
	"clientupdator/server/internal/service"
	"clientupdator/server/internal/utils"
	"clientupdator/server/models"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/utils-go/ngo/collections/generic"
	"github.com/utils-go/ngo/io/directory"
	"github.com/utils-go/ngo/io/file"
	"github.com/utils-go/ngo/io/fileinfo"
	"github.com/utils-go/ngo/io/path"
	"github.com/utils-go/ngo/linq"
)

func GetAllFilesByProjectId(ctx *gin.Context) {
	var projectIdDto struct {
		ProjectId int `json:"projectId"`
	}
	if err := ctx.BindUri(&projectIdDto); err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	projectResult := service.GetProjectById(projectIdDto.ProjectId)
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
	files, err := directory.GetFiles(workDir, "*,*", true)
	//查询文件
	if err != nil {
		ctx.JSON(200, models.NGWithError(err))
		return
	}

	result := generic.NewList[models.FileInfo]()
	for i := 0; i < len(files); i++ {
		//f是全路径
		f := files[i]
		//获取相对路径
		relPath, _ := path.GetRelativePath(workDir, f)
		if linq.From(p.IgnoreFolders).Where(func(ignoreFolder string) bool {
			return path.IsSubPath(relPath, ignoreFolder)
		}).Count() > 0 {
			//忽略的文件夹，跳过
			continue
		}
		if linq.From(p.IgnoreFiles).Where(func(ignoreFile string) bool {
			return ignoreFile == relPath
		}).Count() > 0 {
			//忽略的文件，跳过
			continue
		}

		info := fileinfo.GetFileInfo(f)
		md5Str, err := utils.GetFileMD5(f)
		if err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
		sha256Str, err := utils.GetFileSHA256(f)
		if err != nil {
			ctx.JSON(200, models.NGWithError(err))
			return
		}
		fileInfo := models.FileInfo{
			FileAbsolutePath: f,
			FileRelativePath: relPath,
			LastUpdateTime:   info.LastWriteTime,
			FileSize:         info.Length,
			MD5:              md5Str,
			SHA256:           sha256Str,
		}

		result.Add(fileInfo)
	}

	ctx.JSON(200, models.OKWithData(result.ToArray()))
}

func DownloadFile(ctx *gin.Context) {
	pathStr := ctx.Query("path")
	if !file.Exists(pathStr) {
		ctx.Redirect(http.StatusNotFound, "/404")
		return
	}
	fileName := path.GetFileName(pathStr)
	ctx.Header("Content-Type", "application/octet-stream")
	ctx.Header("Content-Disposition", "attachment; filename="+fileName)
	ctx.Header("Content-Transfer-Encoding", "binary")
	ctx.File(pathStr)
}

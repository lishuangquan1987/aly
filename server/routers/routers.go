package routers

import (
	"time"

	"aly/server/controllers"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {
	group := r.Group("api")
	// 审查 B11：全局 WriteTimeout 置 0 后，普通 JSON 接口统一给 60s 写期限；
	// 下载路由单独注册（不挂本中间件），由 DownloadFile 按文件大小设置期限。
	json := controllers.JSONWriteDeadline(60 * time.Second)
	{
		projectGroup := group.Group("project", json)
		{
			projectGroup.POST("create_project", controllers.CreateProject)
			projectGroup.POST("update_project", controllers.UpdateProject)
			projectGroup.POST("set_force_update", controllers.SetForceUpdate)
			projectGroup.GET("get_all_projects", controllers.GetAllProjects)
			projectGroup.GET("get_project_by_name/:projectName", controllers.GetProjectByName)
			projectGroup.GET("get_project_change_logs/:projectName", controllers.GetProjectChangeLogs)
			projectGroup.POST("delete_project/:projectName", controllers.DeleteProject)
			projectGroup.POST("publish_version", controllers.PublishVersion)
			projectGroup.GET("get_project_os_info/:projectName", controllers.GetProjectOSInfo)
		}

		serverGroup := group.Group("server", json)
		{
			serverGroup.GET("info", controllers.ServerInfo)
		}
		fileGroup := group.Group("file", json)
		{
			fileGroup.POST("upload_file", controllers.UploadFile)
			fileGroup.POST("upload_chunk", controllers.UploadChunk)
			fileGroup.POST("upload_chunk_complete", controllers.UploadChunkComplete)
			// 审查 B6：删除同步链路
			fileGroup.POST("delete_file", controllers.DeleteFile)
			fileGroup.GET("get_all_files/:projectName", controllers.GetAllFilesByProjectName)
			fileGroup.GET("get_release_manifest/:projectName", controllers.GetReleaseManifest)
		}
		// 下载不走 JSON 写期限中间件（按文件大小单独 SetWriteDeadline）
		group.GET("file/download_file", controllers.DownloadFile)
	}
}

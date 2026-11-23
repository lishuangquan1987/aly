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
	// 审查修复（code-review-2026-11-23 #5）：JSON 接口请求体上限 1MB——
	// 上传链路有 ContentLength+MaxBytesReader 三层限制，JSON 组此前裸奔：
	// publish_version 的 logs 数组可提交数百 MB 全量读入并无界入库。
	// 1MB 远超合法负载（几十条变更说明 + 脚本路径）。
	jsonMaxBytes := controllers.JSONMaxBytes(1 << 20)
	// 链式：先限体积再设写期限（顺序无关，均为前置处理）
	jsonGroup := func(h gin.HandlerFunc) gin.HandlerFunc {
		return func(ctx *gin.Context) {
			jsonMaxBytes(ctx)
			h(ctx)
		}
	}(json)
	{
		projectGroup := group.Group("project", jsonGroup)
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

		serverGroup := group.Group("server", jsonGroup)
		{
			serverGroup.GET("info", controllers.ServerInfo)
		}
		fileGroup := group.Group("file", jsonGroup)
		{
			fileGroup.POST("upload_file", controllers.UploadFile)
			fileGroup.POST("upload_chunk", controllers.UploadChunk)
			fileGroup.POST("upload_chunk_complete", controllers.UploadChunkComplete)
			fileGroup.GET("get_all_files/:projectName", controllers.GetAllFilesByProjectName)
			fileGroup.GET("get_release_manifest/:projectName", controllers.GetReleaseManifest)
		}
		// 下载不走 JSON 写期限中间件（按文件大小单独 SetWriteDeadline）
		group.GET("file/download_file", controllers.DownloadFile)
	}
}

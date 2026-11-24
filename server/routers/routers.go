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
		// fileGroup 只挂写期限，不挂 1MB MaxBytes（复审 P1 修复）：
		// upload_file/upload_chunk 的 body 是 multipart 文件载荷（上限 1GB/64MB），
		// 由 handler 自身的 ContentLength 早退 + MaxBytesReader 分层限制；
		// 挂 1MB 组级限制会把 ALY_CHUNK_SIZE 调大后的分片全部截断，
		// 且 MaxBytesError 被误报为"文件过大: 上限 1GB"（报的上限与真实限制不符）。
		fileGroup := group.Group("file", json)
		{
			fileGroup.POST("upload_file", controllers.UploadFile)
			fileGroup.POST("upload_chunk", controllers.UploadChunk)
			// upload_chunk_complete 的 body 只有表单字段（通常 <1KB），挂 1MB 防滥用。
			fileGroup.POST("upload_chunk_complete", controllers.UploadChunkComplete, jsonMaxBytes)
			fileGroup.GET("get_all_files/:projectName", controllers.GetAllFilesByProjectName)
			fileGroup.GET("get_release_manifest/:projectName", controllers.GetReleaseManifest)
		}
		// 下载不走 JSON 写期限中间件（按文件大小单独 SetWriteDeadline）
		group.GET("file/download_file", controllers.DownloadFile)
	}
}

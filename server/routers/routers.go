package routers

import (
	"clientupdator/server/controllers"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {
	group := r.Group("api")
	{
		projectGroup := group.Group("project")
		{
			projectGroup.POST("create_project", controllers.CreateProject)
			projectGroup.POST("update_project", controllers.UpdateProject)
			projectGroup.GET("get_all_projects", controllers.GetAllProjects)
			projectGroup.GET("get_project_change_logs/:projectId", controllers.GetProjectChangeLogs)
			projectGroup.POST("delete_project/:projectId", controllers.DeleteProject)
			projectGroup.POST("publish_version", controllers.PublishVersion)
			projectGroup.GET("get_project_os_info/:projectId", controllers.GetProjectOSInfo)
		}
		fileGroup := group.Group("file")
		{
			fileGroup.POST("upload_file", controllers.UploadFile)
			fileGroup.GET("get_all_files/:projectId", controllers.GetAllFilesByProjectId)
			fileGroup.GET("download_file", controllers.DownloadFile)
		}
	}
}

package routers

import (
	"aly/server/controllers"

	"github.com/gin-gonic/gin"
)

func InitRouter(r *gin.Engine) {
	group := r.Group("api")
	{
		projectGroup := group.Group("project")
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

		serverGroup := group.Group("server")
		{
			serverGroup.GET("info", controllers.ServerInfo)
		}
		fileGroup := group.Group("file")
		{
			fileGroup.POST("upload_file", controllers.UploadFile)
			fileGroup.GET("get_all_files/:projectName", controllers.GetAllFilesByProjectName)
			fileGroup.GET("download_file", controllers.DownloadFile)
		}
	}
}

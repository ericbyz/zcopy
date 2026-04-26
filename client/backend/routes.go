package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func registerRoutes(router *gin.Engine, app *AppState) {
	api := router.Group("/api/v1")
	registerAuthRoutes(api, app)
	registerTaskRoutes(api, app)
	registerSyncRoutes(api, app)
	registerOnDemandRoutes(api, app)
	registerRemoteRoutes(api, app)
	registerFileProviderRoutes(api, app)
	registerSystemRoutes(api, app)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}

func registerAuthRoutes(api *gin.RouterGroup, app *AppState) {
	api.POST("/auth/register", app.proxyRegister)
	api.POST("/auth/login", app.proxyLogin)
	api.POST("/auth/logout", app.logout)
	api.GET("/auth/me", app.proxyMe)
}

func registerTaskRoutes(api *gin.RouterGroup, app *AppState) {
	api.GET("/tasks", app.listTasks)
	api.POST("/tasks", app.createTask)
	api.PUT("/tasks/:id", app.updateTask)
	api.DELETE("/tasks/:id", app.deleteTask)
}

func registerSyncRoutes(api *gin.RouterGroup, app *AppState) {
	api.POST("/tasks/:id/sync", app.syncTaskNow)
	api.POST("/tasks/:id/auto/start", app.startAutoTask)
	api.POST("/tasks/:id/auto/stop", app.stopAutoTask)
}

func registerOnDemandRoutes(api *gin.RouterGroup, app *AppState) {
	api.POST("/tasks/:id/on-demand/release", app.releaseLocalSpace)
	api.POST("/tasks/:id/on-demand/hydrate", app.hydrateFromCloud)
	api.POST("/tasks/:id/on-demand/cfapi/init", app.initTaskCFAPI)
	api.GET("/tasks/:id/on-demand/cfapi/status", app.taskCFAPIStatus)
}

func registerRemoteRoutes(api *gin.RouterGroup, app *AppState) {
	api.GET("/remote/folders", app.listRemoteFolders)
	api.GET("/logs/export", app.exportLogs)
	api.DELETE("/logs", app.clearLogs)
	api.GET("/logs", app.listLogs)
}

func registerFileProviderRoutes(api *gin.RouterGroup, app *AppState) {
	api.GET("/file-provider/tasks/:id/item", app.fileProviderItem)
	api.GET("/file-provider/tasks/:id/children", app.fileProviderChildren)
	api.GET("/file-provider/tasks/:id/content", app.fileProviderContent)
	api.PUT("/file-provider/tasks/:id/content", app.fileProviderPutContent)
	api.PUT("/file-provider/tasks/:id/rename", app.fileProviderRenameItem)
	api.POST("/file-provider/tasks/:id/folder", app.fileProviderCreateFolder)
	api.DELETE("/file-provider/tasks/:id/item", app.fileProviderDeleteItem)
}

func registerSystemRoutes(api *gin.RouterGroup, app *AppState) {
	api.GET("/system/capabilities", app.systemCapabilities)
}

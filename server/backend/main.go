package main

import (
	"log"
	"net/http"
	"os"

	"zcopy-server-backend/config"
	"zcopy-server-backend/database"
	"zcopy-server-backend/handlers"
	"zcopy-server-backend/middleware"
	"zcopy-server-backend/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	config.LoadConfig()

	if err := utils.EnsureDirectoryExists(config.AppConfig.Storage.RootDir); err != nil {
		log.Fatalf("failed to create storage root: %v", err)
	}

	database.InitDB()
	defer database.DB.Close()

	gin.SetMode(config.AppConfig.Server.Mode)

	router := gin.Default()
	router.MaxMultipartMemory = 64 << 20
	router.Use(corsMiddleware())

	api := router.Group("/api/v1")
	{
		authGroup := api.Group("/auth")
		{
			authGroup.POST("/register", handlers.Register)
			authGroup.POST("/login", handlers.Login)
			authGroup.GET("/me", middleware.AuthRequired(), handlers.Me)
		}

		fileGroup := api.Group("/files")
		fileGroup.Use(middleware.AuthRequired())
		{
			fileGroup.GET("", handlers.ListFiles)
			fileGroup.POST("/folder", handlers.CreateFolder)
			fileGroup.POST("/upload", handlers.UploadFile)
			fileGroup.GET("/download", handlers.DownloadFile)
			fileGroup.PUT("/rename", handlers.RenameFile)
			fileGroup.DELETE("", handlers.DeleteFile)
		}

		clientGroup := api.Group("/client")
		clientGroup.Use(middleware.AuthRequired())
		{
			clientGroup.GET("/capabilities", handlers.ClientCapabilities)
		}
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	address := ":" + config.AppConfig.Server.Port
	log.Printf("server started at %s", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}

		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func init() {
	_ = os.Setenv("TZ", "Asia/Shanghai")
}

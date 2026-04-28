package handlers

import (
	"net/http"

	"zcopy-server-backend/serverinfo"

	"github.com/gin-gonic/gin"
)

func GetServerInfo(c *gin.Context) {
	info := serverinfo.GetServerInfo()
	if info == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "服务器信息未初始化"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "ok",
		"data":    info,
	})
}

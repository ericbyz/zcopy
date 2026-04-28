package handlers

import (
	"net/http"

	"zcopy-server-backend/logger"
	"zcopy-server-backend/middleware"
	"zcopy-server-backend/session"

	"github.com/gin-gonic/gin"
)

func ListOnlineClients(sessionMgr *session.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, _ := c.Get("request_id")
		_, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
			return
		}

		clients := sessionMgr.ListOnline()

		logger.Debug("list online clients", "request_id", requestID, "count", len(clients))

		c.JSON(http.StatusOK, gin.H{
			"items": clients,
			"total": len(clients),
		})
	}
}

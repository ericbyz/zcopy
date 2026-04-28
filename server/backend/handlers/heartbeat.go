package handlers

import (
	"fmt"
	"net/http"
	"time"

	"zcopy-server-backend/middleware"
	"zcopy-server-backend/models"
	"zcopy-server-backend/session"

	"github.com/gin-gonic/gin"
)

type HeartbeatRequest struct {
	ActiveTasks int    `json:"activeTasks"`
	ClientIP    string `json:"clientIp"`
}

func ClientHeartbeat(sessionMgr *session.SessionManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := middleware.CurrentUser(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
			return
		}

		var req HeartbeatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			req.ActiveTasks = 0
			req.ClientIP = ""
		}

		clientIP := c.ClientIP()
		if req.ClientIP != "" {
			clientIP = req.ClientIP
		}

		sessionID := fmt.Sprintf("%d-%s", user.ID, clientIP)

		session := models.ClientSession{
			SessionID:     sessionID,
			UserID:        user.ID,
			Username:      user.Username,
			ClientIP:      clientIP,
			ActiveTasks:   req.ActiveTasks,
			LastHeartbeat: time.Now(),
			Status:        "online",
			ConnectedAt:   time.Now(),
		}

		sessionMgr.RegisterOrUpdate(session)

		c.JSON(http.StatusOK, gin.H{
			"message": "heartbeat received",
			"data": gin.H{
				"next_heartbeat_seconds": 10,
			},
		})
	}
}

package middleware

import (
	"fmt"
	"time"

	"zcopy-server-backend/logger"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		c.Set("request_id", requestID)

		start := time.Now()
		method := c.Request.Method
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		clientIP := c.ClientIP()

		c.Next()

		duration := time.Since(start)
		statusCode := c.Writer.Status()

		var userID any
		if user, ok := CurrentUser(c); ok {
			userID = user.ID
		}

		logger.Info("request completed",
			"request_id", requestID,
			"method", method,
			"path", path,
			"query", query,
			"status_code", statusCode,
			"duration_ms", duration.Milliseconds(),
			"client_ip", clientIP,
			"user_id", userID,
		)
	}
}

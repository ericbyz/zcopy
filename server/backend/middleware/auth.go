package middleware

import (
	"errors"
	"net/http"
	"strings"

	"zcopy-server-backend/config"
	"zcopy-server-backend/database"
	"zcopy-server-backend/logger"
	"zcopy-server-backend/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthClaims struct {
	UserID uint `json:"user_id"`
	jwt.RegisteredClaims
}

func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID, _ := c.Get("request_id")
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			logger.Warn("auth failed", "request_id", requestID, "reason", "missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "未提供认证信息"})
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer"))
		if tokenString == "" {
			logger.Warn("auth failed", "request_id", requestID, "reason", "empty bearer token")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "无效的认证信息"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(tokenString, &AuthClaims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.AppConfig.Auth.SecretKey), nil
		})
		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				logger.Info("auth failed", "request_id", requestID, "reason", "token expired")
			} else {
				logger.Warn("auth failed", "request_id", requestID, "reason", "invalid token", "error", err.Error())
			}
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已失效，请重新登录"})
			c.Abort()
			return
		}
		if !token.Valid {
			logger.Warn("auth failed", "request_id", requestID, "reason", "invalid token")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录已失效，请重新登录"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*AuthClaims)
		if !ok {
			logger.Warn("auth failed", "request_id", requestID, "reason", "invalid claims")
			c.JSON(http.StatusUnauthorized, gin.H{"message": "无效的登录凭证"})
			c.Abort()
			return
		}

		user, err := database.DB.FindUserByID(claims.UserID)
		if err != nil {
			logger.Warn("auth failed", "request_id", requestID, "reason", "user not found", "user_id", claims.UserID)
			c.JSON(http.StatusUnauthorized, gin.H{"message": "用户不存在"})
			c.Abort()
			return
		}

		c.Set("user", user)
		c.Next()
	}
}

func CurrentUser(c *gin.Context) (models.User, bool) {
	value, exists := c.Get("user")
	if !exists {
		return models.User{}, false
	}

	user, ok := value.(models.User)
	return user, ok
}

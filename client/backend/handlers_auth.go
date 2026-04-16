package main

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Auth proxy handlers
// ---------------------------------------------------------------------------

func (a *AppState) proxyRegister(c *gin.Context) {
	a.proxyAuthEndpoint(c, "/auth/register", false)
}

func (a *AppState) proxyLogin(c *gin.Context) {
	data, status, err := a.proxyRaw(c.Request.Method, "/auth/login", c.Request.Body, "application/json", "")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	if status >= 200 && status < 300 {
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err == nil {
			if token, ok := payload["token"].(string); ok && token != "" {
				a.setToken(token)
			}
		}
	}
	c.Data(status, "application/json", data)
}

func (a *AppState) proxyMe(c *gin.Context) {
	a.proxyAuthEndpoint(c, "/auth/me", true)
}

func (a *AppState) logout(c *gin.Context) {
	a.setToken("")
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

func (a *AppState) proxyAuthEndpoint(c *gin.Context, path string, withToken bool) {
	token := ""
	if withToken {
		token = a.getToken()
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
			return
		}
	}
	data, status, err := a.proxyRaw(c.Request.Method, path, c.Request.Body, c.GetHeader("Content-Type"), token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	c.Data(status, "application/json", data)
}

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/models"
)

// ---------------------------------------------------------------------------
// Auth proxy handlers
// ---------------------------------------------------------------------------

func (a *AppState) proxyRegister(c *gin.Context) {
	a.pushLog("info", models.BackupTask{Name: "system"}, "", "注册请求")
	a.proxyAuthEndpoint(c, "/auth/register", false)
}

func (a *AppState) proxyLogin(c *gin.Context) {
	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	var reqBody map[string]string
	_ = c.ShouldBindJSON(&reqBody)

	data, status, err := a.proxyRaw(c.Request.Method, "/auth/login", bytes.NewReader(bodyBytes), "application/json", "")
	if err != nil {
		a.pushLog("error", models.BackupTask{Name: "system"}, "", "登录失败: "+err.Error())
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
		username := ""
		if reqBody != nil {
			if u, ok := reqBody["username"]; ok {
				username = u
			} else if u, ok := reqBody["email"]; ok {
				username = u
			}
		}
		a.pushLog("info", models.BackupTask{Name: "system"}, "", "登录成功"+func() string {
			if username != "" {
				return " (" + username + ")"
			}
			return ""
		}())
	} else {
		a.pushLog("warn", models.BackupTask{Name: "system"}, "", "登录失败")
	}
	c.Data(status, "application/json", data)
}

func (a *AppState) proxyMe(c *gin.Context) {
	a.pushLog("debug", models.BackupTask{Name: "system"}, "", "查询当前用户信息")
	a.proxyAuthEndpoint(c, "/auth/me", true)
}

func (a *AppState) logout(c *gin.Context) {
	a.pushLog("info", models.BackupTask{Name: "system"}, "", "用户登出")
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

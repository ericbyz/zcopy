package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/models"
	"zcopy-client-backend/proxy"
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

	var reqBody map[string]any
	_ = c.ShouldBindJSON(&reqBody)

	serverID, _ := reqBody["serverId"].(string)
	delete(reqBody, "serverId")
	remoteBody := bodyBytes
	if len(reqBody) > 0 {
		if encoded, err := json.Marshal(reqBody); err == nil {
			remoteBody = encoded
		}
	}

	client, resolvedServerID, err := a.authClientForServer(serverID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "服务器不存在或未配置"})
		return
	}

	data, status, err := client.RawRequest(c.Request.Method, "/auth/login", bytes.NewReader(remoteBody), "application/json", "")
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
				if resolvedServerID != "" {
					a.multiTokens.SetToken(resolvedServerID, token)
				}
			}
		}
		username := ""
		if reqBody != nil {
			if u, ok := reqBody["username"].(string); ok {
				username = u
			} else if u, ok := reqBody["email"].(string); ok {
				username = u
			} else if u, ok := reqBody["account"].(string); ok {
				username = u
			}
		}
		a.markServerOnline(resolvedServerID)
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
	var req struct {
		ServerID string `json:"serverId"`
	}
	_ = c.ShouldBindJSON(&req)
	a.pushLog("info", models.BackupTask{Name: "system"}, "", "用户登出")
	a.setToken("")
	if req.ServerID != "" {
		a.multiTokens.RemoveToken(req.ServerID)
	}
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

func (a *AppState) authClientForServer(serverID string) (proxy.RemoteClient, string, error) {
	if serverID == "" {
		if defSrv, ok := a.Registry.GetDefault(); ok {
			serverID = defSrv.ID
		}
	}
	if serverID == "" {
		return a.remote, "", nil
	}
	if client, err := a.getClientForServer(serverID); err == nil {
		return client, serverID, nil
	}
	server, ok := a.Registry.GetServer(serverID)
	if !ok {
		return nil, "", errors.New("server not found")
	}
	baseURL := serverAPIBaseURL(server.Address)
	a.multiProxy.AddServer(server.ID, baseURL)
	return proxy.NewHTTPRemoteClient(a.httpc, baseURL), server.ID, nil
}

func (a *AppState) markServerOnline(serverID string) {
	if serverID == "" {
		return
	}
	server, ok := a.Registry.GetServer(serverID)
	if !ok {
		return
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	server.Status = "online"
	server.LastConnectedAt = time.Now().In(loc)
	_ = a.Registry.UpdateServer(server)
}

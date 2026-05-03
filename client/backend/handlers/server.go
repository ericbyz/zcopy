package handlers

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/discovery"
	"zcopy-client-backend/models"
	"zcopy-client-backend/registry"
)

type AddServerRequest struct {
	Name    string `json:"name" binding:"required"`
	Address string `json:"address" binding:"required"` // host:port
	WebURL  string `json:"webUrl"`
}

type UpdateServerRequest struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	WebURL  string `json:"webUrl"`
}

func generateUUID() (string, error) {
	uuid := make([]byte, 16)
	_, err := rand.Read(uuid)
	if err != nil {
		return "", err
	}

	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		uuid[0], uuid[1], uuid[2], uuid[3],
		uuid[4], uuid[5],
		uuid[6], uuid[7],
		uuid[8], uuid[9],
		uuid[10], uuid[11], uuid[12], uuid[13], uuid[14], uuid[15]), nil
}

func ListServers(reg *registry.ServerRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		servers := reg.ListServers()
		c.JSON(http.StatusOK, gin.H{
			"items": servers,
			"total": len(servers),
		})
	}
}

func AddServer(reg *registry.ServerRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req AddServerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		id, err := generateUUID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to generate server ID"})
			return
		}

		loc, _ := time.LoadLocation("Asia/Shanghai")
		now := time.Now().In(loc)

		servers := reg.ListServers()
		isDefault := len(servers) == 0

		server := models.ServerConfig{
			ID:              id,
			Name:            req.Name,
			Address:         req.Address,
			WebURL:          req.WebURL,
			IsDefault:       isDefault,
			Status:          "unknown",
			AddedAt:         now,
			LastConnectedAt: time.Time{},
		}

		if err := reg.AddServer(server); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		c.JSON(http.StatusCreated, gin.H{
			"message": "服务器添加成功",
			"data":    server,
		})
	}
}

func UpdateServer(reg *registry.ServerRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		var req UpdateServerRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
			return
		}

		server, ok := reg.GetServer(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"message": "服务器不存在"})
			return
		}

		if req.Name != "" {
			server.Name = req.Name
		}
		if req.Address != "" {
			server.Address = req.Address
		}
		if req.WebURL != "" {
			server.WebURL = req.WebURL
		}

		if err := reg.UpdateServer(server); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "服务器更新成功",
			"data":    server,
		})
	}
}

func DeleteServer(reg *registry.ServerRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		if err := reg.RemoveServer(id); err != nil {
			if err == registry.ErrServerNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "服务器不存在"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "已删除"})
	}
}

func TestConnection(reg *registry.ServerRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		server, ok := reg.GetServer(id)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"message": "服务器不存在"})
			return
		}

		client := &http.Client{Timeout: 5 * time.Second}
		url := fmt.Sprintf("http://%s/api/v1/server/info", server.Address)
		resp, err := client.Get(url)
		if err != nil {
			server.Status = "offline"
			reg.UpdateServer(server)
			c.JSON(http.StatusBadGateway, gin.H{"message": "无法连接到服务器: " + err.Error()})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			server.Status = "offline"
			reg.UpdateServer(server)
			c.JSON(http.StatusBadGateway, gin.H{"message": fmt.Sprintf("服务器返回错误状态码: %d", resp.StatusCode)})
			return
		}

		loc, _ := time.LoadLocation("Asia/Shanghai")
		server.Status = "online"
		server.LastConnectedAt = time.Now().In(loc)
		reg.UpdateServer(server)

		c.JSON(http.StatusOK, gin.H{"message": "连接成功", "data": server})
	}
}

func ScanServers() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		defer cancel()

		servers, err := discovery.ScanLAN(ctx, 3*time.Second, 1900)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"items": servers,
			"total": len(servers),
		})
	}
}

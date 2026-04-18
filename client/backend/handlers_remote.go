package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/models"
	"zcopy-client-backend/utils"
)

type remoteFileListResponse struct {
	Path  string `json:"path"`
	Items []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		IsDirectory bool   `json:"isDirectory"`
	} `json:"items"`
}

func (a *AppState) listRemoteFolders(c *gin.Context) {
	a.pushLog("debug", models.BackupTask{Name: "system"}, "", "远程目录浏览")
	token := a.getToken()
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}
	currentPath := strings.Trim(filepath.ToSlash(strings.TrimSpace(c.Query("path"))), "/")
	endpoint := "/files"
	if currentPath != "" {
		endpoint += "?path=" + url.QueryEscape(currentPath)
	}
	data, status, err := a.proxyRaw(http.MethodGet, endpoint, nil, "", token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程目录失败: " + err.Error()})
		return
	}
	if status < 200 || status >= 300 {
		upstreamMessage := utils.ParseJSONMessage(data)
		if upstreamMessage == "" {
			upstreamMessage = "读取远程目录失败"
		}
		if status == http.StatusUnauthorized {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "登录状态已失效，请重新登录"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"message": upstreamMessage})
		return
	}

	var payload remoteFileListResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "解析远程目录失败"})
		return
	}

	folders := make([]gin.H, 0)
	for _, item := range payload.Items {
		if !item.IsDirectory {
			continue
		}
		folders = append(folders, gin.H{
			"name": item.Name,
			"path": strings.Trim(filepath.ToSlash(item.Path), "/"),
		})
	}

	parentPath := ""
	if payload.Path != "" {
		parentPath = filepath.ToSlash(filepath.Dir(payload.Path))
		if parentPath == "." {
			parentPath = ""
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"path":       strings.Trim(filepath.ToSlash(payload.Path), "/"),
		"parentPath": strings.Trim(parentPath, "/"),
		"folders":    folders,
	})
}

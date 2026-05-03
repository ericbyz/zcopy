package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-server-backend/logger"
	"zcopy-server-backend/middleware"
	"zcopy-server-backend/syncservice"
)

type syncTaskRequest struct {
	ClientTaskID string `json:"clientTaskId"`
	TaskName     string `json:"taskName"`
	RemotePath   string `json:"remotePath"`
	ConflictMode string `json:"conflictMode"`
	OnDemandSync bool   `json:"onDemandSync"`
}

type syncEventAckRequest struct {
	TaskID  string `json:"taskId"`
	Path    string `json:"path"`
	Type    string `json:"type"`
	Result  string `json:"result"`
	Detail  string `json:"detail"`
	Message string `json:"message"`
}

func UpsertSyncTask(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	var req syncTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}
	req.ClientTaskID = strings.TrimSpace(req.ClientTaskID)
	req.TaskName = strings.TrimSpace(req.TaskName)
	req.RemotePath = strings.Trim(filepath.ToSlash(strings.TrimSpace(req.RemotePath)), "/")
	if req.ClientTaskID == "" || req.TaskName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "任务参数不完整"})
		return
	}
	if occupiedBy, exists := syncservice.Service.OccupiedBy(user.ID, req.RemotePath, req.ClientTaskID); exists {
		c.JSON(http.StatusConflict, gin.H{
			"message":  "该文件服务器文件夹已被其他任务占用",
			"taskId":   occupiedBy.ClientTaskID,
			"taskName": occupiedBy.TaskName,
		})
		return
	}
	if err := syncservice.Service.Upsert(syncservice.SyncTask{
		UserID:       user.ID,
		ClientTaskID: req.ClientTaskID,
		TaskName:     req.TaskName,
		RemotePath:   req.RemotePath,
		ConflictMode: req.ConflictMode,
		OnDemandSync: req.OnDemandSync,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存同步任务失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "同步任务注册成功"})
}

func DeleteSyncTask(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}
	taskID := strings.TrimSpace(c.Param("taskId"))
	if taskID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "任务标识不能为空"})
		return
	}
	if err := syncservice.Service.Remove(user.ID, taskID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "同步任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "同步任务已删除"})
}

func ListSyncFolders(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}
	excludeTaskID := strings.TrimSpace(c.Query("excludeTaskId"))
	relativePath, absolutePath, err := resolveUserPath(user.ID, c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}
	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"path":    relativePath,
				"folders": []gin.H{},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取目录失败"})
		return
	}

	folders := make([]gin.H, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		itemPath := filepath.ToSlash(filepath.Join(relativePath, entry.Name()))
		folder := gin.H{
			"name": entry.Name(),
			"path": itemPath,
		}
		if occupiedBy, exists := syncservice.Service.OccupiedBy(user.ID, itemPath, excludeTaskID); exists {
			folder["occupied"] = true
			folder["taskId"] = occupiedBy.ClientTaskID
			folder["taskName"] = occupiedBy.TaskName
		} else {
			folder["occupied"] = false
		}
		folders = append(folders, folder)
	}

	sort.Slice(folders, func(i, j int) bool {
		return strings.ToLower(folders[i]["name"].(string)) < strings.ToLower(folders[j]["name"].(string))
	})

	currentInfo := gin.H{"occupied": false}
	if occupiedBy, exists := syncservice.Service.OccupiedBy(user.ID, relativePath, excludeTaskID); exists {
		currentInfo["occupied"] = true
		currentInfo["taskId"] = occupiedBy.ClientTaskID
		currentInfo["taskName"] = occupiedBy.TaskName
	}

	c.JSON(http.StatusOK, gin.H{
		"path":         relativePath,
		"folders":      folders,
		"currentOwner": currentInfo,
	})
}

func StreamSyncEvents(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}
	eventCh, unsubscribe := syncservice.Service.Subscribe(user.ID)
	defer unsubscribe()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "当前连接不支持流式输出"})
		return
	}

	logger.Info("sync event stream connected", "user_id", user.ID)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			logger.Info("sync event stream disconnected", "user_id", user.ID)
			return
		case evt := <-eventCh:
			payload, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			_, _ = c.Writer.Write([]byte("event: sync\n"))
			_, _ = c.Writer.Write([]byte("data: "))
			_, _ = c.Writer.Write(payload)
			_, _ = c.Writer.Write([]byte("\n\n"))
			flusher.Flush()
		case <-ticker.C:
			_, _ = c.Writer.Write([]byte(": ping\n\n"))
			flusher.Flush()
		}
	}
}

func AckSyncEvent(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}
	var req syncEventAckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}
	req.Result = strings.TrimSpace(req.Result)
	req.Detail = strings.TrimSpace(req.Detail)
	if req.Result == "" {
		req.Result = "unknown"
	}
	taskName := ""
	remotePath := ""
	if task, exists := syncservice.Service.GetTask(user.ID, req.TaskID); exists {
		taskName = task.TaskName
		remotePath = task.RemotePath
	}
	logArgs := []any{
		"user_id", user.ID,
		"task_id", strings.TrimSpace(req.TaskID),
		"task_name", taskName,
		"remote_path", remotePath,
		"path", strings.TrimSpace(req.Path),
		"event_type", strings.TrimSpace(req.Type),
		"result", req.Result,
	}
	if req.Detail != "" {
		logArgs = append(logArgs, "detail", req.Detail)
	}
	if req.Result == "success" {
		logger.Info("sync event applied on client", logArgs...)
	} else {
		logger.Warn("sync event apply failed on client", logArgs...)
	}
	c.JSON(http.StatusOK, gin.H{"message": "同步事件结果已记录"})
}

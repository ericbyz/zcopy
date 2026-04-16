package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/models"
	"zcopy-client-backend/utils"
)

func (a *AppState) listTasks(c *gin.Context) {
	c.JSON(200, gin.H{"items": a.store.List()})
}

func (a *AppState) createTask(c *gin.Context) {
	var req models.BackupTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "请求参数格式错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.LocalPath = filepath.Clean(strings.TrimSpace(req.LocalPath))
	req.RemotePath = utils.NormalizeRemote(req.RemotePath)
	if req.Name == "" || req.LocalPath == "" {
		c.JSON(400, gin.H{"message": "任务名称和本地目录不能为空"})
		return
	}
	info, err := os.Stat(req.LocalPath)
	if err != nil || !info.IsDir() {
		c.JSON(400, gin.H{"message": "本地目录不存在或不可用"})
		return
	}
	now := time.Now()
	req.ID = "task-" + now.Format("20060102150405.000000000")
	req.Status = "idle"
	req.CreatedAt = now
	req.UpdatedAt = now
	if err := a.store.Upsert(req); err != nil {
		c.JSON(500, gin.H{"message": "保存任务失败: " + err.Error()})
		return
	}
	if req.AutoBackup {
		if err := a.watcher.StartWatcher(req.ID, req); err != nil {
			c.JSON(500, gin.H{"message": "任务已创建，但自动同步启动失败: " + err.Error()})
			return
		}
	}
	if err := a.maybeInitTaskFileProvider(req); err != nil {
		c.JSON(500, gin.H{"message": "任务已创建，但按需同步初始化失败: " + err.Error(), "task": req})
		return
	}
	c.JSON(201, gin.H{"message": "创建成功", "task": req})
}

func (a *AppState) updateTask(c *gin.Context) {
	id := c.Param("id")
	oldTask, ok := a.store.Get(id)
	if !ok {
		c.JSON(404, gin.H{"message": "任务不存在"})
		return
	}
	var req models.BackupTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "请求参数格式错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.LocalPath = filepath.Clean(strings.TrimSpace(req.LocalPath))
	req.RemotePath = utils.NormalizeRemote(req.RemotePath)
	if req.Name == "" || req.LocalPath == "" {
		c.JSON(400, gin.H{"message": "任务名称和本地目录不能为空"})
		return
	}
	info, err := os.Stat(req.LocalPath)
	if err != nil || !info.IsDir() {
		c.JSON(400, gin.H{"message": "本地目录不存在或不可用"})
		return
	}
	req.ID = oldTask.ID
	req.CreatedAt = oldTask.CreatedAt
	req.LastSyncAt = oldTask.LastSyncAt
	req.LastError = oldTask.LastError
	req.Status = oldTask.Status
	req.SyncReport = oldTask.SyncReport
	req.UpdatedAt = time.Now()
	if err := a.store.Upsert(req); err != nil {
		c.JSON(500, gin.H{"message": "更新任务失败"})
		return
	}
	if req.AutoBackup {
		if err := a.watcher.StartWatcher(req.ID, req); err != nil {
			c.JSON(500, gin.H{"message": "任务已更新，但自动同步启动失败: " + err.Error()})
			return
		}
	} else {
		a.watcher.StopWatcher(req.ID)
	}
	if err := a.maybeInitTaskFileProvider(req); err != nil {
		c.JSON(500, gin.H{"message": "任务已更新，但按需同步初始化失败: " + err.Error(), "task": req})
		return
	}
	c.JSON(200, gin.H{"message": "更新成功", "task": req})
}

func (a *AppState) deleteTask(c *gin.Context) {
	id := c.Param("id")
	a.watcher.StopWatcher(id)
	if err := a.store.Remove(id); err != nil {
		c.JSON(404, gin.H{"message": "任务不存在"})
		return
	}
	c.JSON(200, gin.H{"message": "删除成功"})
}

func (a *AppState) maybeInitTaskFileProvider(task models.BackupTask) error {
	if !task.OnDemandSync || runtime.GOOS != "darwin" || !a.fileProviderAvailable() {
		return nil
	}
	_, err := a.initTaskFileProvider(task)
	return err
}

package main

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/models"
)

func (a *AppState) listTasks(c *gin.Context) {
	a.pushLog("debug", models.BackupTask{Name: "system"}, "", "任务列表查询")
	c.JSON(200, gin.H{"items": a.store.List()})
}

func (a *AppState) createTask(c *gin.Context) {
	var req models.BackupTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"message": "请求参数格式错误"})
		return
	}
	if err := a.validateAndPrepareTask(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
		return
	}
	token := a.getToken()
	if token == "" {
		c.JSON(401, gin.H{"message": "请先登录客户端"})
		return
	}
	if req.RemotePath != "" {
		if err := a.ensureRemotePath(req.RemotePath, token); err != nil {
			c.JSON(500, gin.H{"message": "创建远程目录失败: " + err.Error()})
			return
		}
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
	if err := a.upsertRemoteSyncTask(req); err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	if !req.CloudOnly && req.AutoBackup {
		if err := a.watcher.StartWatcher(req.ID, req); err != nil {
			c.JSON(500, gin.H{"message": "任务已创建，但自动同步启动失败: " + err.Error()})
			return
		}
	}
	if err := a.maybeInitTaskFileProvider(req); err != nil {
		c.JSON(500, gin.H{"message": "任务已创建，但按需同步初始化失败: " + err.Error(), "task": req})
		return
	}
	a.pushLog("info", req, req.LocalPath, "任务已创建: "+req.Name+", 本地路径: "+req.LocalPath+", 远程路径: "+req.RemotePath)
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
	if err := a.validateAndPrepareTask(&req); err != nil {
		c.JSON(400, gin.H{"message": err.Error()})
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
	if isSyncTask(oldTask) && !isSyncTask(req) {
		if err := a.deleteRemoteSyncTask(oldTask); err != nil {
			c.JSON(500, gin.H{"message": "更新任务成功，但移除服务端同步任务失败: " + err.Error()})
			return
		}
	}
	if err := a.upsertRemoteSyncTask(req); err != nil {
		c.JSON(409, gin.H{"message": err.Error()})
		return
	}
	if !req.CloudOnly && req.AutoBackup {
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
	a.pushLog("info", req, req.LocalPath, "任务已更新: "+id)
	c.JSON(200, gin.H{"message": "更新成功", "task": req})
}

func (a *AppState) deleteTask(c *gin.Context) {
	id := c.Param("id")
	task, _ := a.store.Get(id)
	a.watcher.StopWatcher(id)
	if err := a.deleteRemoteSyncTask(task); err != nil {
		c.JSON(500, gin.H{"message": "删除服务端同步任务失败: " + err.Error()})
		return
	}
	if err := a.store.Remove(id); err != nil {
		c.JSON(404, gin.H{"message": "任务不存在"})
		return
	}
	a.pushLog("warn", task, "", "任务已删除: "+task.Name)
	c.JSON(200, gin.H{"message": "删除成功"})
}

func (a *AppState) maybeInitTaskFileProvider(task models.BackupTask) error {
	if !task.OnDemandSync || runtime.GOOS != "darwin" || !a.fileProviderAvailable() {
		return nil
	}
	_, err := a.initTaskFileProvider(task)
	return err
}

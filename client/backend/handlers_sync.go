package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *AppState) syncTaskNow(c *gin.Context) {
	id := c.Param("id")
	if err := a.syncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "同步失败: " + err.Error()})
		return
	}
	task, _ := a.store.Get(id)
	c.JSON(http.StatusOK, gin.H{"message": "同步成功", "task": task})
}

func (a *AppState) startAutoTask(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	task.AutoBackup = true
	task.UpdatedAt = time.Now()
	if err := a.store.Upsert(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存任务失败"})
		return
	}
	if err := a.watcher.StartWatcher(id, task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "启动自动同步失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "自动同步已启动"})
}

func (a *AppState) stopAutoTask(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	task.AutoBackup = false
	task.UpdatedAt = time.Now()
	if err := a.store.Upsert(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存任务失败"})
		return
	}
	a.watcher.StopWatcher(id)
	c.JSON(http.StatusOK, gin.H{"message": "自动同步已停止"})
}

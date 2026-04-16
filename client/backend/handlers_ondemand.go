package main

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	syncpkg "zcopy-client-backend/sync"
)

func (a *AppState) releaseLocalSpace(c *gin.Context) {
	id := c.Param("id")
	result, err := a.syncer.ReleaseLocalSpace(id)
	if errors.Is(err, syncpkg.ErrTaskNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	if errors.Is(err, syncpkg.ErrSnapshotMissing) {
		c.JSON(400, gin.H{"message": "请先完成一次同步后再释放本地空间"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"message": "扫描本地目录失败"})
		return
	}
	c.JSON(200, gin.H{
		"message":       "本地空间释放完成",
		"releasedFiles": result.ReleasedFiles,
		"releasedBytes": result.ReleasedBytes,
		"skippedFiles":  result.SkippedFiles,
		"task":          result.Task,
	})
}

func (a *AppState) hydrateFromCloud(c *gin.Context) {
	id := c.Param("id")
	task, err := a.syncer.HydrateFromCloud(id)
	if errors.Is(err, syncpkg.ErrTaskNotFound) {
		c.JSON(404, gin.H{"message": "任务不存在"})
		return
	}
	if errors.Is(err, syncpkg.ErrUnauthorized) {
		c.JSON(401, gin.H{"message": "未登录"})
		return
	}
	if errors.Is(err, syncpkg.ErrSnapshotMissing) {
		c.JSON(400, gin.H{"message": "没有可恢复的云端文件，请先完成同步"})
		return
	}
	if err != nil {
		c.JSON(500, gin.H{"message": "下载失败", "task": task})
		return
	}
	c.JSON(200, gin.H{"message": "云端文件下载完成", "task": task})
}

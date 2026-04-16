package main

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (a *AppState) listLogs(c *gin.Context) {
	taskID := strings.TrimSpace(c.Query("taskId"))
	level := strings.TrimSpace(strings.ToLower(c.Query("level")))
	limit := 100
	if c.Query("limit") != "" {
		if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	result := a.logs.List(taskID, level, limit)
	c.JSON(http.StatusOK, gin.H{"items": result})
}

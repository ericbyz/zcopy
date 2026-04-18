package main

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	logpkg "zcopy-client-backend/log"
)

func (a *AppState) listLogs(c *gin.Context) {
	var query logpkg.ListQuery

	pageStr := strings.TrimSpace(c.Query("page"))
	pageSizeStr := strings.TrimSpace(c.Query("pageSize"))
	limitStr := strings.TrimSpace(c.Query("limit"))

	page := 1
	if pageStr != "" {
		if v, err := strconv.Atoi(pageStr); err == nil && v >= 1 {
			page = v
		}
	}

	pageSize := 20
	if pageSizeStr != "" {
		if v, err := strconv.Atoi(pageSizeStr); err == nil {
			pageSize = v
		}
	} else if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			pageSize = v
		}
	}

	if pageSize < 1 {
		pageSize = 1
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query.Page = page
	query.PageSize = pageSize

	query.TaskID = strings.TrimSpace(c.Query("taskId"))
	query.Level = strings.TrimSpace(strings.ToLower(c.Query("level")))
	query.Keyword = strings.TrimSpace(c.Query("keyword"))

	if startTimeStr := strings.TrimSpace(c.Query("startTime")); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			query.StartTime = &t
		}
	}
	if endTimeStr := strings.TrimSpace(c.Query("endTime")); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			query.EndTime = &t
		}
	}

	result, err := a.logs.Query(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "查询日志失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    result.Items,
		"total":    result.Total,
		"page":     result.Page,
		"pageSize": result.PageSize,
	})
}

func (a *AppState) exportLogs(c *gin.Context) {
	var query logpkg.ListQuery

	query.TaskID = strings.TrimSpace(c.Query("taskId"))
	query.Level = strings.TrimSpace(strings.ToLower(c.Query("level")))
	query.Keyword = strings.TrimSpace(c.Query("keyword"))
	query.Page = 1
	query.PageSize = 10000

	if startTimeStr := strings.TrimSpace(c.Query("startTime")); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			query.StartTime = &t
		}
	}
	if endTimeStr := strings.TrimSpace(c.Query("endTime")); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			query.EndTime = &t
		}
	}

	reader, err := a.logs.Export(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "导出日志失败: " + err.Error()})
		return
	}
	if closer, ok := reader.(io.ReadCloser); ok {
		defer closer.Close()
	}

	filename := "zcopy-logs-" + time.Now().Format("20060102150405") + ".jsonl"
	c.Header("Content-Type", "application/x-jsonlines")
	c.Header("Content-Disposition", "attachment; filename="+filename)

	io.Copy(c.Writer, reader)
}

package handlers

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"zcopy-server-backend/config"
	"zcopy-server-backend/middleware"

	"github.com/gin-gonic/gin"
)

type LogEntry struct {
	ID         string `json:"id"`
	Time       string `json:"time"`
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	UserID     string `json:"userId,omitempty"`
	Operation  string `json:"operation,omitempty"`
	Path       string `json:"path,omitempty"`
	Method     string `json:"method,omitempty"`
	StatusCode int    `json:"statusCode,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
	RequestID  string `json:"requestId,omitempty"`
	Error      string `json:"error,omitempty"`
}

type rawLogEntry struct {
	Time       string `json:"time"`
	Level      string `json:"level"`
	Msg        string `json:"msg"`
	UserID     string `json:"user_id,omitempty"`
	Operation  string `json:"operation,omitempty"`
	Path       string `json:"path,omitempty"`
	Method     string `json:"method,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
	Error      string `json:"error,omitempty"`
}

func ListLogs(c *gin.Context) {
	logDir := config.AppConfig.Log.Dir

	currentUser, _ := middleware.CurrentUser(c)
	currentUserID := strconv.FormatUint(uint64(currentUser.ID), 10)

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	level := c.Query("level")
	keyword := c.Query("keyword")
	startTimeStr := c.Query("startTime")
	endTimeStr := c.Query("endTime")

	var startTime, endTime *time.Time
	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		}
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取日志目录失败"})
		return
	}

	var logFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") || strings.Contains(name, ".jsonl.") {
			logFiles = append(logFiles, filepath.Join(logDir, name))
		}
	}

	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := os.Stat(logFiles[i])
		infoJ, _ := os.Stat(logFiles[j])
		return infoI.ModTime().After(infoJ.ModTime())
	})

	var allEntries []LogEntry
	lineCounter := 0

	for _, filePath := range logFiles {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		fileName := filepath.Base(filePath)

		for scanner.Scan() {
			lineCounter++
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var raw rawLogEntry
			if err := json.Unmarshal(line, &raw); err != nil {
				continue
			}

			logTime, err := time.Parse(time.RFC3339, raw.Time)
			if err != nil {
				continue
			}

			if level != "" && !strings.EqualFold(raw.Level, level) {
				continue
			}

			if keyword != "" {
				hasKeyword := strings.Contains(strings.ToLower(raw.Msg), strings.ToLower(keyword)) ||
					strings.Contains(strings.ToLower(raw.Path), strings.ToLower(keyword))
				if !hasKeyword {
					continue
				}
			}

			if startTime != nil && logTime.Before(*startTime) {
				continue
			}

			if endTime != nil && logTime.After(*endTime) {
				continue
			}

			if raw.UserID != "" && raw.UserID != currentUserID {
				continue
			}

			entry := LogEntry{
				ID:         "log-" + strconv.Itoa(lineCounter) + "-" + fileName,
				Time:       raw.Time,
				Level:      raw.Level,
				Msg:        raw.Msg,
				UserID:     raw.UserID,
				Operation:  raw.Operation,
				Path:       raw.Path,
				Method:     raw.Method,
				StatusCode: raw.StatusCode,
				DurationMs: raw.DurationMs,
				RequestID:  raw.RequestID,
				Error:      raw.Error,
			}

			allEntries = append(allEntries, entry)
		}
	}

	total := len(allEntries)
	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= total {
		c.JSON(http.StatusOK, gin.H{
			"items":    []LogEntry{},
			"total":    total,
			"page":     page,
			"pageSize": pageSize,
		})
		return
	}

	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"items":    allEntries[start:end],
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func ExportLogs(c *gin.Context) {
	logDir := config.AppConfig.Log.Dir

	currentUser, _ := middleware.CurrentUser(c)
	currentUserID := strconv.FormatUint(uint64(currentUser.ID), 10)

	level := c.Query("level")
	keyword := c.Query("keyword")
	startTimeStr := c.Query("startTime")
	endTimeStr := c.Query("endTime")

	var startTime, endTime *time.Time
	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		}
	}

	entries, err := os.ReadDir(logDir)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取日志目录失败"})
		return
	}

	var logFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") || strings.Contains(name, ".jsonl.") {
			logFiles = append(logFiles, filepath.Join(logDir, name))
		}
	}

	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := os.Stat(logFiles[i])
		infoJ, _ := os.Stat(logFiles[j])
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	c.Header("Content-Type", "application/jsonl")
	c.Header("Content-Disposition", "attachment; filename=logs.jsonl")

	for _, filePath := range logFiles {
		file, err := os.Open(filePath)
		if err != nil {
			continue
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var raw rawLogEntry
			if err := json.Unmarshal(line, &raw); err != nil {
				continue
			}

			logTime, err := time.Parse(time.RFC3339, raw.Time)
			if err != nil {
				continue
			}

			if level != "" && !strings.EqualFold(raw.Level, level) {
				continue
			}

			if keyword != "" {
				hasKeyword := strings.Contains(strings.ToLower(raw.Msg), strings.ToLower(keyword)) ||
					strings.Contains(strings.ToLower(raw.Path), strings.ToLower(keyword))
				if !hasKeyword {
					continue
				}
			}

			if startTime != nil && logTime.Before(*startTime) {
				continue
			}

			if endTime != nil && logTime.After(*endTime) {
				continue
			}

			if raw.UserID != "" && raw.UserID != currentUserID {
				continue
			}

			if _, err := c.Writer.Write(line); err != nil {
				return
			}
			if _, err := c.Writer.Write([]byte("\n")); err != nil {
				return
			}
		}
	}
}

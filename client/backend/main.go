package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Server struct {
		Port string `yaml:"port"`
		Mode string `yaml:"mode"`
	} `yaml:"server"`
	FileServer struct {
		BaseURL string `yaml:"base_url"`
	} `yaml:"file_server"`
	Storage struct {
		DataDir string `yaml:"data_dir"`
	} `yaml:"storage"`
}

type BackupTask struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	LocalPath    string      `json:"localPath"`
	RemotePath   string      `json:"remotePath"`
	AutoBackup   bool        `json:"autoBackup"`
	OnDemandSync bool        `json:"onDemandSync"`
	Status       string      `json:"status"`
	LastError    string      `json:"lastError"`
	LastSyncAt   *time.Time  `json:"lastSyncAt"`
	SyncReport   *SyncReport `json:"syncReport,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

type SyncReport struct {
	State            string     `json:"state"`
	Mode             string     `json:"mode"`
	Message          string     `json:"message"`
	TotalFiles       int        `json:"totalFiles"`
	UploadedFiles    int        `json:"uploadedFiles"`
	FailedFiles      int        `json:"failedFiles"`
	TotalBytes       int64      `json:"totalBytes"`
	TransferredBytes int64      `json:"transferredBytes"`
	SpeedBytesPerSec int64      `json:"speedBytesPerSec"`
	FailedFilePaths  []string   `json:"failedFilePaths,omitempty"`
	StartedAt        time.Time  `json:"startedAt"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
}

type TransferLog struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	TaskName  string    `json:"taskName"`
	Level     string    `json:"level"`
	FilePath  string    `json:"filePath,omitempty"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type TaskSnapshot struct {
	Files map[string]FileFingerprint `json:"files"`
}

type FileFingerprint struct {
	Size    int64 `json:"size"`
	ModUnix int64 `json:"modUnix"`
}

type LocalFileItem struct {
	AbsPath string
	RelPath string
	Size    int64
	ModUnix int64
}

type TaskStore struct {
	mu    sync.RWMutex
	path  string
	tasks []BackupTask
}

type WatchController struct {
	stopCh chan struct{}
	doneCh chan struct{}
}

type AppState struct {
	cfg            AppConfig
	httpc          *http.Client
	tokenMu        sync.RWMutex
	token          string
	store          *TaskStore
	watchMu        sync.Mutex
	watchJobs      map[string]*WatchController
	syncMu         sync.Mutex
	syncing        map[string]bool
	logMu          sync.Mutex
	logs           []TransferLog
	fpBridgeURL    string
	fpBridgeToken  string
	webdavBaseURL  string
	webdavUsername string
	webdavPassword string
}

type remoteFileListResponse struct {
	Path  string `json:"path"`
	Items []struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		IsDirectory bool   `json:"isDirectory"`
	} `json:"items"`
}

func main() {
	cfg := loadConfig()
	if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}
	store := &TaskStore{
		path: filepath.Join(cfg.Storage.DataDir, "tasks.json"),
	}
	if err := store.load(); err != nil {
		log.Fatalf("failed to load task store: %v", err)
	}

	gin.SetMode(cfg.Server.Mode)
	app := &AppState{
		cfg:           cfg,
		httpc:         &http.Client{Timeout: 60 * time.Second},
		store:         store,
		watchJobs:     make(map[string]*WatchController),
		syncing:       make(map[string]bool),
		fpBridgeURL:   strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_FP_BRIDGE_URL")),
		fpBridgeToken: strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_FP_BRIDGE_TOKEN")),
	}
	if err := app.startWebDAVServer(); err != nil {
		log.Fatalf("failed to start file provider webdav server: %v", err)
	}

	app.restoreAutoWatchers()

	router := gin.Default()
	router.Use(cors())

	api := router.Group("/api/v1")
	api.POST("/auth/register", app.proxyRegister)
	api.POST("/auth/login", app.proxyLogin)
	api.POST("/auth/logout", app.logout)
	api.GET("/auth/me", app.proxyMe)

	api.GET("/tasks", app.listTasks)
	api.POST("/tasks", app.createTask)
	api.PUT("/tasks/:id", app.updateTask)
	api.DELETE("/tasks/:id", app.deleteTask)
	api.POST("/tasks/:id/sync", app.syncTaskNow)
	api.POST("/tasks/:id/auto/start", app.startAutoTask)
	api.POST("/tasks/:id/auto/stop", app.stopAutoTask)
	api.POST("/tasks/:id/on-demand/release", app.releaseLocalSpace)
	api.POST("/tasks/:id/on-demand/hydrate", app.hydrateFromCloud)
	api.POST("/tasks/:id/on-demand/cfapi/init", app.initTaskCFAPI)
	api.GET("/tasks/:id/on-demand/cfapi/status", app.taskCFAPIStatus)
	api.GET("/remote/folders", app.listRemoteFolders)
	api.GET("/file-provider/tasks/:id/item", app.fileProviderItem)
	api.GET("/file-provider/tasks/:id/children", app.fileProviderChildren)
	api.GET("/file-provider/tasks/:id/content", app.fileProviderContent)
	api.PUT("/file-provider/tasks/:id/content", app.fileProviderPutContent)
	api.POST("/file-provider/tasks/:id/folder", app.fileProviderCreateFolder)
	api.DELETE("/file-provider/tasks/:id/item", app.fileProviderDeleteItem)
	api.GET("/logs", app.listLogs)
	api.GET("/system/capabilities", app.systemCapabilities)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	addr := ":" + cfg.Server.Port
	log.Printf("client backend started at %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("failed to run client backend: %v", err)
	}
}

func loadConfig() AppConfig {
	configPath := resolveConfigPath()
	buf, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read config: %v", err)
	}
	var cfg AppConfig
	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		log.Fatalf("failed to parse config: %v", err)
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8090"
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "debug"
	}
	if cfg.FileServer.BaseURL == "" {
		cfg.FileServer.BaseURL = "http://localhost:8890/api/v1"
	}
	if cfg.Storage.DataDir == "" {
		cfg.Storage.DataDir = "./data"
	}
	if envDataDir := strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_DATA_DIR")); envDataDir != "" {
		cfg.Storage.DataDir = envDataDir
	}
	return cfg
}

func resolveConfigPath() string {
	if envPath := strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_CONFIG")); envPath != "" {
		return envPath
	}
	candidates := []string{
		filepath.Join("config", "config.yaml"),
		filepath.Join(".", "config", "config.yaml"),
	}
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidates = append(candidates, filepath.Join(exeDir, "config", "config.yaml"))
		if parent := filepath.Dir(exeDir); parent != exeDir {
			candidates = append(candidates, filepath.Join(parent, "config", "config.yaml"))
		}
	}
	if lookedPath, err := exec.LookPath("zcopy-client-backend.exe"); err == nil {
		binDir := filepath.Dir(lookedPath)
		candidates = append(candidates, filepath.Join(binDir, "config", "config.yaml"))
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join("config", "config.yaml")
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (a *AppState) proxyRegister(c *gin.Context) {
	a.proxyAuthEndpoint(c, "/auth/register", false)
}

func (a *AppState) proxyLogin(c *gin.Context) {
	data, status, err := a.proxyRaw(http.MethodPost, "/auth/login", c.Request.Body, "application/json", "")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	if status >= 200 && status < 300 {
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err == nil {
			if token, ok := payload["token"].(string); ok && token != "" {
				a.setToken(token)
			}
		}
	}
	c.Data(status, "application/json", data)
}

func (a *AppState) proxyMe(c *gin.Context) {
	a.proxyAuthEndpoint(c, "/auth/me", true)
}

func (a *AppState) logout(c *gin.Context) {
	a.setToken("")
	c.JSON(http.StatusOK, gin.H{"message": "已退出登录"})
}

func (a *AppState) proxyAuthEndpoint(c *gin.Context, path string, withToken bool) {
	token := ""
	if withToken {
		token = a.getToken()
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
			return
		}
	}
	data, status, err := a.proxyRaw(c.Request.Method, path, c.Request.Body, c.GetHeader("Content-Type"), token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": err.Error()})
		return
	}
	c.Data(status, "application/json", data)
}

func (a *AppState) proxyRaw(method string, path string, body io.Reader, contentType string, token string) ([]byte, int, error) {
	var copiedBody []byte
	if body != nil {
		buf, _ := io.ReadAll(body)
		copiedBody = buf
	}
	req, err := http.NewRequest(method, strings.TrimRight(a.cfg.FileServer.BaseURL, "/")+path, bytes.NewReader(copiedBody))
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.httpc.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

func (a *AppState) listTasks(c *gin.Context) {
	tasks := a.store.list()
	c.JSON(http.StatusOK, gin.H{"items": tasks})
}

func (a *AppState) listLogs(c *gin.Context) {
	taskID := strings.TrimSpace(c.Query("taskId"))
	level := strings.TrimSpace(strings.ToLower(c.Query("level")))
	limit := 100
	if c.Query("limit") != "" {
		if v, err := strconv.Atoi(c.Query("limit")); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	a.logMu.Lock()
	defer a.logMu.Unlock()
	result := make([]TransferLog, 0, limit)
	for i := len(a.logs) - 1; i >= 0 && len(result) < limit; i-- {
		item := a.logs[i]
		if taskID != "" && item.TaskID != taskID {
			continue
		}
		if level != "" && strings.ToLower(item.Level) != level {
			continue
		}
		result = append(result, item)
	}
	c.JSON(http.StatusOK, gin.H{"items": result})
}

func (a *AppState) pushLog(level string, task BackupTask, filePath string, message string) {
	a.logMu.Lock()
	defer a.logMu.Unlock()
	entry := TransferLog{
		ID:        "log-" + time.Now().Format("20060102150405.000000000"),
		TaskID:    task.ID,
		TaskName:  task.Name,
		Level:     level,
		FilePath:  filePath,
		Message:   message,
		CreatedAt: time.Now(),
	}
	a.logs = append(a.logs, entry)
	if len(a.logs) > 2000 {
		a.logs = a.logs[len(a.logs)-2000:]
	}
}

func (a *AppState) listRemoteFolders(c *gin.Context) {
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
		upstreamMessage := parseJSONMessage(data)
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

func parseJSONMessage(data []byte) string {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	if msg, ok := payload["message"].(string); ok {
		return msg
	}
	return ""
}

func (a *AppState) createTask(c *gin.Context) {
	var req BackupTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.LocalPath = filepath.Clean(strings.TrimSpace(req.LocalPath))
	req.RemotePath = normalizeRemote(req.RemotePath)
	if req.Name == "" || req.LocalPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "任务名称和本地目录不能为空"})
		return
	}
	info, err := os.Stat(req.LocalPath)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"message": "本地目录不存在或不可用"})
		return
	}
	now := time.Now()
	req.ID = "task-" + now.Format("20060102150405.000000000")
	req.Status = "idle"
	req.CreatedAt = now
	req.UpdatedAt = now
	if err := a.store.upsert(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存任务失败: " + err.Error()})
		return
	}
	if req.AutoBackup {
		if err := a.startWatcher(req.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "任务已创建，但自动同步启动失败: " + err.Error()})
			return
		}
	}
	if err := a.maybeInitTaskFileProvider(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "任务已创建，但按需同步初始化失败: " + err.Error(), "task": req})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"message": "创建成功", "task": req})
}

func (a *AppState) updateTask(c *gin.Context) {
	id := c.Param("id")
	oldTask, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	var req BackupTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.LocalPath = filepath.Clean(strings.TrimSpace(req.LocalPath))
	req.RemotePath = normalizeRemote(req.RemotePath)
	if req.Name == "" || req.LocalPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "任务名称和本地目录不能为空"})
		return
	}
	info, err := os.Stat(req.LocalPath)
	if err != nil || !info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"message": "本地目录不存在或不可用"})
		return
	}
	req.ID = oldTask.ID
	req.CreatedAt = oldTask.CreatedAt
	req.LastSyncAt = oldTask.LastSyncAt
	req.LastError = oldTask.LastError
	req.Status = oldTask.Status
	req.SyncReport = oldTask.SyncReport
	req.UpdatedAt = time.Now()
	if err := a.store.upsert(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "更新任务失败"})
		return
	}
	if req.AutoBackup {
		if err := a.startWatcher(req.ID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "任务已更新，但自动同步启动失败: " + err.Error()})
			return
		}
	} else {
		a.stopWatcher(req.ID)
	}
	if err := a.maybeInitTaskFileProvider(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "任务已更新，但按需同步初始化失败: " + err.Error(), "task": req})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功", "task": req})
}

func (a *AppState) maybeInitTaskFileProvider(task BackupTask) error {
	if !task.OnDemandSync || runtime.GOOS != "darwin" || !a.fileProviderAvailable() {
		return nil
	}
	_, err := a.initTaskFileProvider(task)
	return err
}

func (a *AppState) deleteTask(c *gin.Context) {
	id := c.Param("id")
	a.stopWatcher(id)
	if err := a.store.remove(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (a *AppState) syncTaskNow(c *gin.Context) {
	id := c.Param("id")
	if err := a.syncTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "同步失败: " + err.Error()})
		return
	}
	task, _ := a.store.get(id)
	c.JSON(http.StatusOK, gin.H{"message": "同步成功", "task": task})
}

func (a *AppState) startAutoTask(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	task.AutoBackup = true
	task.UpdatedAt = time.Now()
	if err := a.store.upsert(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存任务失败"})
		return
	}
	if err := a.startWatcher(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "启动自动同步失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "自动同步已启动"})
}

func (a *AppState) stopAutoTask(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	task.AutoBackup = false
	task.UpdatedAt = time.Now()
	if err := a.store.upsert(task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存任务失败"})
		return
	}
	a.stopWatcher(id)
	c.JSON(http.StatusOK, gin.H{"message": "自动同步已停止"})
}

func (a *AppState) releaseLocalSpace(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	snapshot := a.loadSnapshot(task.ID)
	if len(snapshot.Files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请先完成一次同步后再释放本地空间"})
		return
	}
	files, err := collectLocalFiles(task.LocalPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "扫描本地目录失败"})
		return
	}
	currentMap := make(map[string]LocalFileItem, len(files))
	for _, item := range files {
		currentMap[item.RelPath] = item
	}
	releasedFiles := 0
	releasedBytes := int64(0)
	skipped := 0
	for rel, fp := range snapshot.Files {
		item, exists := currentMap[rel]
		if !exists {
			continue
		}
		if item.Size != fp.Size || item.ModUnix != fp.ModUnix {
			skipped++
			continue
		}
		if err := os.Remove(item.AbsPath); err != nil {
			skipped++
			a.pushLog("error", task, rel, "释放本地空间失败: "+err.Error())
			continue
		}
		releasedFiles++
		releasedBytes += item.Size
	}
	now := time.Now()
	task.Status = "idle"
	task.SyncReport = &SyncReport{
		State:      "idle",
		Mode:       "on_demand",
		Message:    "本地空间释放完成",
		StartedAt:  now,
		FinishedAt: &now,
	}
	task.UpdatedAt = now
	_ = a.store.upsert(task)
	a.pushLog("info", task, "", "本地空间释放完成")
	c.JSON(http.StatusOK, gin.H{
		"message":       "本地空间释放完成",
		"releasedFiles": releasedFiles,
		"releasedBytes": releasedBytes,
		"skippedFiles":  skipped,
		"task":          task,
	})
}

func (a *AppState) hydrateFromCloud(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	token := a.getToken()
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}
	snapshot := a.loadSnapshot(task.ID)
	if len(snapshot.Files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "没有可恢复的云端文件，请先完成同步"})
		return
	}
	keys := make([]string, 0, len(snapshot.Files))
	for rel := range snapshot.Files {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	report := &SyncReport{
		State:      "syncing",
		Mode:       "on_demand_hydrate",
		Message:    "开始下载云端文件",
		StartedAt:  time.Now(),
		TotalFiles: len(keys),
	}
	task.Status = "syncing"
	task.SyncReport = report
	task.UpdatedAt = time.Now()
	_ = a.store.upsert(task)

	currentFiles, _ := collectLocalFiles(task.LocalPath)
	currentMap := make(map[string]LocalFileItem, len(currentFiles))
	for _, item := range currentFiles {
		currentMap[item.RelPath] = item
	}
	var firstErr error
	failed := make([]string, 0)
	for _, rel := range keys {
		fp := snapshot.Files[rel]
		localPath := filepath.Join(task.LocalPath, filepath.FromSlash(rel))
		if item, exists := currentMap[rel]; exists && item.Size == fp.Size && item.ModUnix == fp.ModUnix {
			report.UploadedFiles++
			report.TransferredBytes += item.Size
			continue
		}
		remoteFile := normalizeRemote(filepath.ToSlash(filepath.Join(task.RemotePath, rel)))
		if err := a.downloadRemoteFile(remoteFile, localPath, token); err != nil {
			report.FailedFiles++
			failed = append(failed, rel)
			if firstErr == nil {
				firstErr = err
			}
			a.pushLog("error", task, rel, "下载云端文件失败: "+err.Error())
		} else {
			report.UploadedFiles++
			report.TransferredBytes += fp.Size
		}
		report.SpeedBytesPerSec = calcSpeed(report.TransferredBytes, report.StartedAt)
		task.UpdatedAt = time.Now()
		_ = a.store.upsert(task)
	}
	now := time.Now()
	if firstErr != nil {
		task.Status = "error"
		task.LastError = firstErr.Error()
		report.State = "failed"
		report.Message = "下载云端文件失败"
		report.FailedFilePaths = failed
		report.FinishedAt = &now
		task.SyncReport = report
		task.UpdatedAt = now
		_ = a.store.upsert(task)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "下载失败", "task": task})
		return
	}
	task.Status = "idle"
	task.LastError = ""
	task.LastSyncAt = &now
	task.SyncReport = &SyncReport{
		State:      "idle",
		Mode:       "on_demand_hydrate",
		Message:    "云端文件下载完成",
		StartedAt:  now,
		FinishedAt: &now,
	}
	task.UpdatedAt = now
	_ = a.store.upsert(task)
	a.pushLog("info", task, "", "云端文件下载完成")
	c.JSON(http.StatusOK, gin.H{"message": "云端文件下载完成", "task": task})
}

func (a *AppState) initTaskCFAPI(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	if runtime.GOOS == "darwin" {
		if !a.fileProviderAvailable() {
			c.JSON(http.StatusBadRequest, gin.H{"message": "当前 mac 客户端未启用 File Provider 支持"})
			return
		}
		status, err := a.initTaskFileProvider(task)
		if err != nil {
			a.pushLog("error", task, "", "注册 File Provider 失败: "+err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"message": "注册 File Provider 失败: " + err.Error()})
			return
		}
		a.pushLog("info", task, "", "File Provider 域注册成功")
		c.JSON(http.StatusOK, gin.H{
			"message":    "File Provider 域注册成功",
			"mountPath":  status.MountPath,
			"logPath":    status.LogPath,
			"registered": status.Registered,
		})
		return
	}
	if runtime.GOOS != "windows" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "仅 Windows 支持 Cloud Files API"})
		return
	}
	if err := registerWindowsSyncRoot(task.ID, task.Name, task.LocalPath); err != nil {
		a.pushLog("error", task, "", "注册 Cloud Files 同步根失败: "+err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "注册 Cloud Files 同步根失败: " + err.Error()})
		return
	}
	a.pushLog("info", task, "", "Cloud Files 同步根注册成功")
	c.JSON(http.StatusOK, gin.H{"message": "Cloud Files 同步根注册成功"})
}

func (a *AppState) taskCFAPIStatus(c *gin.Context) {
	id := c.Param("id")
	task, ok := a.store.get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return
	}
	if runtime.GOOS == "darwin" {
		if !a.fileProviderAvailable() {
			c.JSON(http.StatusOK, gin.H{
				"supported":  false,
				"registered": false,
				"reason":     "mac File Provider bridge 未就绪",
			})
			return
		}
		status, err := a.getTaskFileProviderStatus(task)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"supported":  true,
				"registered": false,
				"reason":     err.Error(),
				"rootId":     syncRootID(task.ID),
				"mode":       "file_provider",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"supported":  true,
			"registered": status.Registered,
			"reason":     status.Reason,
			"rootId":     syncRootID(task.ID),
			"mountPath":  status.MountPath,
			"logPath":    status.LogPath,
			"mode":       "file_provider",
		})
		return
	}
	if runtime.GOOS != "windows" {
		c.JSON(http.StatusOK, gin.H{"supported": false, "registered": false})
		return
	}
	registered, reason := isSyncRootRegistered(task.ID)
	c.JSON(http.StatusOK, gin.H{
		"supported":  true,
		"registered": registered,
		"reason":     reason,
		"rootId":     syncRootID(task.ID),
	})
}

func syncRootID(taskID string) string {
	var builder strings.Builder
	builder.WriteString("ZCopy.")
	for _, r := range taskID {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '.' || r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	return builder.String()
}

func registerWindowsSyncRoot(taskID string, taskName string, localPath string) error {
	script := `
param(
  [Parameter(Mandatory = $true)][string]$RootPath,
  [Parameter(Mandatory = $true)][string]$RootId,
  [Parameter(Mandatory = $true)][string]$DisplayName
)
Set-StrictMode -Version Latest
if (-not (Test-Path -LiteralPath $RootPath)) {
  throw "同步根路径不存在: $RootPath"
}
Add-Type -AssemblyName System.Runtime.WindowsRuntime
$null = [Windows.Storage.Provider.StorageProviderSyncRootManager, Windows.Storage, ContentType=WindowsRuntime]
if (-not [Windows.Storage.Provider.StorageProviderSyncRootManager]::IsSupported()) {
  throw "当前系统不支持 StorageProviderSyncRootManager"
}
$folderOp = [Windows.Storage.StorageFolder]::GetFolderFromPathAsync($RootPath)
$folder = [System.Runtime.InteropServices.WindowsRuntime.WindowsRuntimeMarshal]::AsTask($folderOp).GetAwaiter().GetResult()
$info = [Windows.Storage.Provider.StorageProviderSyncRootInfo]::new()
$info.Id = $RootId
$info.Path = $folder
$info.DisplayNameResource = $DisplayName
$info.IconResource = "$env:SystemRoot\System32\shell32.dll,3"
$info.HydrationPolicy = [Windows.Storage.Provider.StorageProviderHydrationPolicy]::Progressive
$info.HydrationPolicyModifier = [Windows.Storage.Provider.StorageProviderHydrationPolicyModifier]::AutoDehydrationAllowed
$info.PopulationPolicy = [Windows.Storage.Provider.StorageProviderPopulationPolicy]::Full
$info.InSyncPolicy = [Windows.Storage.Provider.StorageProviderInSyncPolicy]::FileCreationTime -bor [Windows.Storage.Provider.StorageProviderInSyncPolicy]::DirectoryCreationTime -bor [Windows.Storage.Provider.StorageProviderInSyncPolicy]::FileLastWriteTime -bor [Windows.Storage.Provider.StorageProviderInSyncPolicy]::DirectoryLastWriteTime
$info.Version = "1.0.0"
$info.ShowSiblingsAsGroup = $false
try {
  $existing = [Windows.Storage.Provider.StorageProviderSyncRootManager]::GetSyncRootInformationForId($RootId)
  if ($existing) {
    [Windows.Storage.Provider.StorageProviderSyncRootManager]::Unregister($RootId)
  }
} catch {
}
[Windows.Storage.Provider.StorageProviderSyncRootManager]::Register($info)
`
	tempFile, err := os.CreateTemp("", "zcopy-cfapi-*.ps1")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempPath)
	if err := os.WriteFile(tempPath, []byte(script), 0644); err != nil {
		return err
	}
	cmd := exec.Command(
		"powershell",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Sta",
		"-File", tempPath,
		"-RootPath", localPath,
		"-RootId", syncRootID(taskID),
		"-DisplayName", "ZCopy "+taskName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

func isSyncRootRegistered(taskID string) (bool, string) {
	keyPath := `HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\SyncRootManager\` + syncRootID(taskID)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "if (Test-Path '"+keyPath+"') { Write-Output 'yes' } else { Write-Output 'no' }")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, strings.TrimSpace(string(out))
	}
	return strings.TrimSpace(string(out)) == "yes", strings.TrimSpace(string(out))
}

func (a *AppState) systemCapabilities(c *gin.Context) {
	onDemandSupport := runtime.GOOS == "windows" || a.fileProviderAvailable()
	onDemandMode := "incremental-upload"
	if runtime.GOOS == "darwin" && onDemandSupport {
		onDemandMode = "file_provider"
	}
	c.JSON(http.StatusOK, gin.H{
		"os":              runtime.GOOS,
		"onDemandSupport": onDemandSupport,
		"onDemandMode":    onDemandMode,
	})
}

func (a *AppState) setToken(token string) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	a.token = token
}

func (a *AppState) getToken() string {
	a.tokenMu.RLock()
	defer a.tokenMu.RUnlock()
	return a.token
}

func (a *AppState) restoreAutoWatchers() {
	for _, task := range a.store.list() {
		if task.AutoBackup {
			if err := a.startWatcher(task.ID); err != nil {
				log.Printf("failed to restore watcher for %s: %v", task.ID, err)
			}
		}
	}
}

func (a *AppState) startWatcher(taskID string) error {
	task, ok := a.store.get(taskID)
	if !ok {
		return errors.New("任务不存在")
	}
	a.watchMu.Lock()
	if ctrl, exists := a.watchJobs[taskID]; exists {
		close(ctrl.stopCh)
		delete(a.watchJobs, taskID)
	}
	ctrl := &WatchController{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	a.watchJobs[taskID] = ctrl
	a.watchMu.Unlock()

	go a.runWatcher(task, ctrl)
	go func() {
		_ = a.syncTask(taskID)
	}()
	return nil
}

func (a *AppState) stopWatcher(taskID string) {
	a.watchMu.Lock()
	ctrl, ok := a.watchJobs[taskID]
	if ok {
		close(ctrl.stopCh)
		delete(a.watchJobs, taskID)
	}
	a.watchMu.Unlock()
}

func (a *AppState) runWatcher(task BackupTask, ctrl *WatchController) {
	defer close(ctrl.doneCh)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()

	err = filepath.Walk(task.LocalPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			_ = watcher.Add(path)
		}
		return nil
	})
	if err != nil {
		return
	}

	timer := time.NewTimer(24 * time.Hour)
	timer.Stop()
	pending := false
	for {
		select {
		case <-ctrl.stopCh:
			return
		case evt := <-watcher.Events:
			if evt.Op&(fsnotify.Create) != 0 {
				if info, statErr := os.Stat(evt.Name); statErr == nil && info.IsDir() {
					_ = watcher.Add(evt.Name)
				}
			}
			if !pending {
				pending = true
				timer.Reset(2 * time.Second)
			}
		case <-timer.C:
			pending = false
			_ = a.syncTask(task.ID)
		case <-watcher.Errors:
		}
	}
}

func (a *AppState) syncTask(taskID string) error {
	task, ok := a.store.get(taskID)
	if !ok {
		return errors.New("任务不存在")
	}
	token := a.getToken()
	if token == "" {
		return errors.New("请先登录客户端")
	}

	a.syncMu.Lock()
	if a.syncing[taskID] {
		a.syncMu.Unlock()
		return nil
	}
	a.syncing[taskID] = true
	a.syncMu.Unlock()

	defer func() {
		a.syncMu.Lock()
		delete(a.syncing, taskID)
		a.syncMu.Unlock()
	}()

	mode := "full"
	if task.OnDemandSync {
		mode = "on_demand"
	}
	report := &SyncReport{
		State:     "syncing",
		Mode:      mode,
		Message:   "准备同步",
		StartedAt: time.Now(),
	}
	task.Status = "syncing"
	task.SyncReport = report
	task.UpdatedAt = time.Now()
	_ = a.store.upsert(task)

	files, err := collectLocalFiles(task.LocalPath)
	if err != nil {
		task.Status = "error"
		task.LastError = err.Error()
		task.UpdatedAt = time.Now()
		report.State = "failed"
		report.Message = "扫描本地目录失败"
		finishedAt := time.Now()
		report.FinishedAt = &finishedAt
		_ = a.store.upsert(task)
		return err
	}
	snapshot := TaskSnapshot{Files: map[string]FileFingerprint{}}
	if task.OnDemandSync {
		snapshot = a.loadSnapshot(task.ID)
	}
	pendingFiles := make([]LocalFileItem, 0, len(files))
	for _, item := range files {
		shouldUpload := true
		if task.OnDemandSync {
			if fp, exists := snapshot.Files[item.RelPath]; exists && fp.Size == item.Size && fp.ModUnix == item.ModUnix {
				shouldUpload = false
			}
		}
		if shouldUpload {
			pendingFiles = append(pendingFiles, item)
			report.TotalFiles++
			report.TotalBytes += item.Size
		}
	}
	if task.OnDemandSync && len(pendingFiles) == 0 {
		now := time.Now()
		report.State = "idle"
		report.Message = "按需同步：未发现变更"
		report.FinishedAt = &now
		task.Status = "idle"
		task.LastError = ""
		task.LastSyncAt = &now
		task.SyncReport = &SyncReport{
			State:      "idle",
			Mode:       mode,
			Message:    "按需同步：无变更",
			StartedAt:  now,
			FinishedAt: &now,
		}
		task.UpdatedAt = now
		return a.store.upsert(task)
	}
	report.Message = "扫描完成，开始传输"
	_ = a.store.upsert(task)

	if err := a.ensureRemotePath(task.RemotePath, token); err != nil {
		task.Status = "error"
		task.LastError = err.Error()
		task.UpdatedAt = time.Now()
		report.State = "failed"
		report.Message = "创建远程目录失败"
		finishedAt := time.Now()
		report.FinishedAt = &finishedAt
		_ = a.store.upsert(task)
		return err
	}

	var firstErr error
	failedFilePaths := make([]string, 0)
	for _, item := range pendingFiles {
		remoteDir := normalizeRemote(filepath.ToSlash(filepath.Join(task.RemotePath, filepath.Dir(item.RelPath))))
		if remoteDir == "." {
			remoteDir = ""
		}
		if err := a.uploadFile(item.AbsPath, remoteDir, token); err != nil {
			report.FailedFiles++
			failedFilePaths = append(failedFilePaths, item.RelPath)
			a.pushLog("error", task, item.RelPath, err.Error())
			if firstErr == nil {
				firstErr = err
			}
		} else {
			report.UploadedFiles++
			report.TransferredBytes += item.Size
		}
		snapshot.Files[item.RelPath] = FileFingerprint{
			Size:    item.Size,
			ModUnix: item.ModUnix,
		}
		report.SpeedBytesPerSec = calcSpeed(report.TransferredBytes, report.StartedAt)
		report.Message = "同步进行中"
		task.UpdatedAt = time.Now()
		_ = a.store.upsert(task)
	}
	for _, item := range files {
		if _, exists := snapshot.Files[item.RelPath]; !exists {
			snapshot.Files[item.RelPath] = FileFingerprint{Size: item.Size, ModUnix: item.ModUnix}
		}
	}
	if task.OnDemandSync {
		_ = a.saveSnapshot(task.ID, snapshot)
	}

	now := time.Now()
	task.LastSyncAt = &now
	task.UpdatedAt = now
	report.FinishedAt = &now
	if firstErr != nil {
		task.Status = "error"
		task.LastError = firstErr.Error()
		report.State = "failed"
		report.Message = "同步失败"
		report.FailedFilePaths = failedFilePaths
		a.pushLog("error", task, "", "任务同步失败")
	} else {
		task.Status = "idle"
		task.LastError = ""
		task.SyncReport = &SyncReport{
			State:      "idle",
			Mode:       mode,
			Message:    "同步完成",
			StartedAt:  now,
			FinishedAt: &now,
		}
		a.pushLog("info", task, "", "任务同步完成")
		return a.store.upsert(task)
	}
	task.SyncReport = report
	return a.store.upsert(task)
}

func collectLocalFiles(root string) ([]LocalFileItem, error) {
	items := make([]LocalFileItem, 0)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		items = append(items, LocalFileItem{
			AbsPath: path,
			RelPath: rel,
			Size:    info.Size(),
			ModUnix: info.ModTime().Unix(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}

func calcSpeed(transferred int64, startedAt time.Time) int64 {
	elapsed := time.Since(startedAt).Seconds()
	if elapsed <= 0 {
		return 0
	}
	return int64(float64(transferred) / elapsed)
}

func (a *AppState) snapshotPath(taskID string) string {
	return filepath.Join(a.cfg.Storage.DataDir, "snapshots", taskID+".json")
}

func (a *AppState) loadSnapshot(taskID string) TaskSnapshot {
	path := a.snapshotPath(taskID)
	buf, err := os.ReadFile(path)
	if err != nil {
		return TaskSnapshot{Files: map[string]FileFingerprint{}}
	}
	var snapshot TaskSnapshot
	if err := json.Unmarshal(buf, &snapshot); err != nil {
		return TaskSnapshot{Files: map[string]FileFingerprint{}}
	}
	if snapshot.Files == nil {
		snapshot.Files = map[string]FileFingerprint{}
	}
	return snapshot
}

func (a *AppState) saveSnapshot(taskID string, snapshot TaskSnapshot) error {
	path := a.snapshotPath(taskID)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	buf, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0644)
}

func (a *AppState) ensureRemotePath(path string, token string) error {
	clean := normalizeRemote(path)
	if clean == "" {
		return nil
	}
	parts := strings.Split(clean, "/")
	parent := ""
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		body, _ := json.Marshal(gin.H{
			"path": parent,
			"name": p,
		})
		_, status, err := a.proxyRaw(http.MethodPost, "/files/folder", bytes.NewReader(body), "application/json", token)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return errors.New("创建远程目录失败")
		}
		if parent == "" {
			parent = p
		} else {
			parent += "/" + p
		}
	}
	return nil
}

func (a *AppState) uploadFile(localFile string, remoteDir string, token string) error {
	f, err := os.Open(localFile)
	if err != nil {
		return err
	}
	defer f.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("path", remoteDir); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("file", filepath.Base(localFile))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	_, status, err := a.proxyRaw(http.MethodPost, "/files/upload", bytes.NewReader(body.Bytes()), writer.FormDataContentType(), token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return errors.New("上传文件失败")
	}
	return nil
}

func (a *AppState) downloadRemoteFile(remotePath string, localPath string, token string) error {
	endpoint := strings.TrimRight(a.cfg.FileServer.BaseURL, "/") + "/files/download?path=" + url.QueryEscape(normalizeRemote(remotePath))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		msg := parseJSONMessage(buf)
		if msg == "" {
			msg = "下载远程文件失败"
		}
		return errors.New(msg)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, resp.Body); err != nil {
		return err
	}
	return nil
}

func normalizeRemote(path string) string {
	p := strings.TrimSpace(filepath.ToSlash(path))
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimSuffix(p, "/")
	if p == "." {
		return ""
	}
	return p
}

func (s *TaskStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.tasks = []BackupTask{}
			return nil
		}
		return err
	}
	if len(buf) == 0 {
		s.tasks = []BackupTask{}
		return nil
	}
	return json.Unmarshal(buf, &s.tasks)
}

func (s *TaskStore) flushLocked() error {
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

func (s *TaskStore) list() []BackupTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]BackupTask, len(s.tasks))
	copy(items, s.tasks)
	return items
}

func (s *TaskStore) get(id string) (BackupTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.ID == id {
			return t, true
		}
	}
	return BackupTask{}, false
}

func (s *TaskStore) upsert(task BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].ID == task.ID {
			s.tasks[i] = task
			return s.flushLocked()
		}
	}
	s.tasks = append(s.tasks, task)
	return s.flushLocked()
}

func (s *TaskStore) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := -1
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("not found")
	}
	s.tasks = append(s.tasks[:index], s.tasks[index+1:]...)
	return s.flushLocked()
}

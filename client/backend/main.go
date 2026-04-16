package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/auth"
	"zcopy-client-backend/config"
	fileproviderpkg "zcopy-client-backend/fileprovider"
	logpkg "zcopy-client-backend/log"
	"zcopy-client-backend/middleware"
	"zcopy-client-backend/models"
	"zcopy-client-backend/proxy"
	"zcopy-client-backend/store"
	syncpkg "zcopy-client-backend/sync"
	"zcopy-client-backend/watcher"
)

type AppState struct {
	cfg     models.AppConfig
	httpc   *http.Client
	store   *store.TaskStore
	tokens  *auth.InMemoryTokenManager
	remote  *proxy.HTTPRemoteClient
	logs    *logpkg.RingBufferLogStore
	watcher *watcher.FSNotifyWatchManager
	syncer  syncpkg.SyncEngine
	fp      *fileproviderpkg.Service
}

func (a *AppState) getToken() string  { return a.tokens.GetToken() }
func (a *AppState) setToken(t string) { a.tokens.SetToken(t) }
func (a *AppState) proxyRaw(method, path string, body io.Reader, contentType, token string) ([]byte, int, error) {
	return a.remote.RawRequest(method, path, body, contentType, token)
}
func (a *AppState) pushLog(level string, task models.BackupTask, filePath, message string) {
	a.logs.Push(level, task, filePath, message)
}
func (a *AppState) ensureRemotePath(path, token string) error {
	return a.remote.EnsureRemotePath(path, token)
}
func (a *AppState) downloadRemoteFile(remotePath, localPath, token string) error {
	return a.remote.DownloadRemoteFile(remotePath, localPath, token)
}

func main() {
	cfg := config.LoadConfig()
	if err := os.MkdirAll(cfg.Storage.DataDir, 0755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	taskStore := store.New(filepath.Join(cfg.Storage.DataDir, "tasks.json"))
	if err := taskStore.Load(); err != nil {
		log.Fatalf("failed to load task store: %v", err)
	}

	httpc := &http.Client{Timeout: 60 * time.Second}
	tokens := auth.NewInMemoryTokenManager()
	remote := proxy.NewHTTPRemoteClient(httpc, cfg.FileServer.BaseURL)
	logs := logpkg.NewRingBufferLogStore()
	snapshotDir := filepath.Join(cfg.Storage.DataDir, "snapshots")
	syncer := syncpkg.NewEngine(taskStore, tokens, remote, logs, snapshotDir)
	fp := fileproviderpkg.New(
		cfg,
		httpc,
		taskStore,
		tokens,
		remote,
		strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_FP_BRIDGE_URL")),
		strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_FP_BRIDGE_TOKEN")),
	)

	app := &AppState{
		cfg:    cfg,
		httpc:  httpc,
		store:  taskStore,
		tokens: tokens,
		remote: remote,
		logs:   logs,
		syncer: syncer,
		fp:     fp,
	}

	app.watcher = watcher.NewFSNotifyWatchManager(taskStore, app.syncTask)

	if err := app.fp.StartWebDAVServer(); err != nil {
		log.Fatalf("failed to start file provider webdav server: %v", err)
	}

	app.watcher.RestoreAutoWatchers()

	gin.SetMode(cfg.Server.Mode)
	router := gin.Default()
	router.Use(middleware.CORS())
	registerRoutes(router, app)

	addr := ":" + cfg.Server.Port
	log.Printf("client backend started at %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("failed to run client backend: %v", err)
	}
}

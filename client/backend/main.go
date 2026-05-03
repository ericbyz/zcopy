package main

import (
	"io"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/auth"
	"zcopy-client-backend/config"
	fileproviderpkg "zcopy-client-backend/fileprovider"
	logpkg "zcopy-client-backend/log"
	"zcopy-client-backend/middleware"
	"zcopy-client-backend/models"
	"zcopy-client-backend/proxy"
	"zcopy-client-backend/registry"
	"zcopy-client-backend/store"
	syncpkg "zcopy-client-backend/sync"
	"zcopy-client-backend/watcher"
)

type AppState struct {
	cfg         models.AppConfig
	httpc       *http.Client
	store       *store.TaskStore
	tokens      *auth.InMemoryTokenManager
	multiTokens *auth.MultiServerTokenManager
	remote      *proxy.HTTPRemoteClient
	multiProxy  *proxy.MultiServerProxy
	logs        logpkg.LogStore
	watcher     *watcher.FSNotifyWatchManager
	syncer      syncpkg.SyncEngine
	fp          *fileproviderpkg.Service
	Registry    *registry.ServerRegistry

	remoteSyncMu     sync.Mutex
	remoteSyncTimers map[string]*time.Timer
}

func (a *AppState) getToken() string { return a.tokens.GetToken() }
func (a *AppState) setToken(t string) {
	a.tokens.SetToken(t)
	if strings.TrimSpace(t) != "" {
		go a.restoreRemoteSyncTasks()
	}
}
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

func (a *AppState) getClientForServer(serverID string) (proxy.RemoteClient, error) {
	if serverID == "" {
		if defSrv, ok := a.Registry.GetDefault(); ok {
			serverID = defSrv.ID
		}
	}
	return a.multiProxy.GetClient(serverID)
}

func (a *AppState) getTokenForServer(serverID string) string {
	if serverID == "" {
		if defSrv, ok := a.Registry.GetDefault(); ok {
			serverID = defSrv.ID
		}
	}
	if token, ok := a.multiTokens.GetToken(serverID); ok && token != "" {
		return token
	}
	return a.tokens.GetToken()
}

func extractAddress(baseURL string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL
	}
	return u.Host
}

func serverAPIBaseURL(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	return strings.TrimRight(address, "/") + "/api/v1"
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		AddSource: true,
	})))
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
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
	logDir := filepath.Join(cfg.Storage.DataDir, "logs")
	logs, err := logpkg.NewFileLogStore(logDir)
	if err != nil {
		log.Fatalf("failed to create log store: %v", err)
	}
	snapshotDir := filepath.Join(cfg.Storage.DataDir, "snapshots")

	serverRegistry, err := registry.NewServerRegistry(cfg.Storage.DataDir)
	if err != nil {
		log.Fatalf("failed to initialize server registry: %v", err)
	}

	servers := serverRegistry.ListServers()
	if len(servers) == 0 {
		addr := extractAddress(cfg.FileServer.BaseURL)
		loc, _ := time.LoadLocation("Asia/Shanghai")
		defaultServer := models.ServerConfig{
			ID:        "default",
			Name:      "默认服务器",
			Address:   addr,
			IsDefault: true,
			Status:    "unknown",
			AddedAt:   time.Now().In(loc),
		}
		if err := serverRegistry.AddServer(defaultServer); err != nil {
			log.Fatalf("failed to add default server: %v", err)
		}
	}

	if defServer, ok := serverRegistry.GetDefault(); ok {
		if err := registry.MigrateTasksToServer(taskStore, defServer.ID); err != nil {
			slog.Warn("task migration partial failure", "error", err)
		}
	}

	multiTokens := auth.NewMultiServerTokenManager(cfg.Storage.DataDir)
	if err := multiTokens.Load(); err != nil {
		slog.Warn("failed to load multi-server tokens", "error", err)
	}
	if token := tokens.GetToken(); token != "" {
		if defSrv, ok := serverRegistry.GetDefault(); ok {
			multiTokens.SetToken(defSrv.ID, token)
		}
	}
	// Restore in-memory token from persisted per-server tokens.
	// This ensures getToken() returns a valid token after a restart
	// without requiring the user to re-login.
	if tokens.GetToken() == "" {
		for _, srv := range serverRegistry.ListServers() {
			if t, ok := multiTokens.GetToken(srv.ID); ok && t != "" {
				tokens.SetToken(t)
				break
			}
		}
	}

	multiProxy := proxy.NewMultiServerProxy(multiTokens)
	for _, srv := range serverRegistry.ListServers() {
		multiProxy.AddServer(srv.ID, serverAPIBaseURL(srv.Address))
	}

	syncer := syncpkg.NewEngine(taskStore, multiTokens, multiProxy, logs, snapshotDir)
	fp := fileproviderpkg.New(
		cfg,
		httpc,
		taskStore,
		tokens,
		remote,
		logs,
		strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_FP_BRIDGE_URL")),
		strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_FP_BRIDGE_TOKEN")),
	)

	app := &AppState{
		cfg:              cfg,
		httpc:            httpc,
		store:            taskStore,
		tokens:           tokens,
		multiTokens:      multiTokens,
		remote:           remote,
		multiProxy:       multiProxy,
		logs:             logs,
		syncer:           syncer,
		fp:               fp,
		Registry:         serverRegistry,
		remoteSyncTimers: make(map[string]*time.Timer),
	}

	app.watcher = watcher.NewFSNotifyWatchManager(taskStore, app.syncTask)

	go func() {
		if err := app.pruneFileProviderDomains(); err != nil {
			slog.Warn("failed to prune File Provider domains", "error", err)
		}
	}()

	app.watcher.RestoreAutoWatchers()
	app.startRemoteEventLoop()

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

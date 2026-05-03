package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"zcopy-client-backend/auth"
	logpkg "zcopy-client-backend/log"
	"zcopy-client-backend/models"
	"zcopy-client-backend/proxy"
	"zcopy-client-backend/registry"
	"zcopy-client-backend/store"
)

func TestProxyLoginRoutesToSelectedServer(t *testing.T) {
	gin.SetMode(gin.TestMode)

	hitServerA := false
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitServerA = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer serverA.Close()

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("unexpected remote path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"token-b","message":"登录成功"}`))
	}))
	defer serverB.Close()

	dataDir := t.TempDir()
	reg, err := registry.NewServerRegistry(dataDir)
	if err != nil {
		t.Fatalf("NewServerRegistry: %v", err)
	}
	now := time.Now()
	if err := reg.AddServer(models.ServerConfig{ID: "server-a", Name: "A", Address: serverA.Listener.Addr().String(), IsDefault: true, AddedAt: now}); err != nil {
		t.Fatalf("add server-a: %v", err)
	}
	if err := reg.AddServer(models.ServerConfig{ID: "server-b", Name: "B", Address: serverB.Listener.Addr().String(), AddedAt: now}); err != nil {
		t.Fatalf("add server-b: %v", err)
	}

	tokens := auth.NewInMemoryTokenManager()
	multiTokens := auth.NewMultiServerTokenManager(filepath.Join(dataDir, "tokens"))
	multiProxy := proxy.NewMultiServerProxy(multiTokens)
	multiProxy.AddServer("server-a", serverAPIBaseURL(serverA.Listener.Addr().String()))
	multiProxy.AddServer("server-b", serverAPIBaseURL(serverB.Listener.Addr().String()))

	app := &AppState{
		httpc:       serverB.Client(),
		store:       store.New(filepath.Join(dataDir, "tasks.json")),
		tokens:      tokens,
		multiTokens: multiTokens,
		remote:      proxy.NewHTTPRemoteClient(serverA.Client(), serverA.URL),
		multiProxy:  multiProxy,
		logs:        logpkg.NewRingBufferLogStore(),
		Registry:    reg,
	}

	router := gin.New()
	router.POST("/auth/login", app.proxyLogin)
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"serverId":"server-b","account":"admin","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("proxyLogin status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if hitServerA {
		t.Fatal("login request was sent to the default server")
	}
	if got := tokens.GetToken(); got != "token-b" {
		t.Fatalf("default token = %q, want token-b", got)
	}
	if got, ok := multiTokens.GetToken("server-b"); !ok || got != "token-b" {
		t.Fatalf("server-b token = %q/%v, want token-b/true", got, ok)
	}
}

func TestServerAPIBaseURL(t *testing.T) {
	tests := map[string]string{
		"localhost:8890":          "http://localhost:8890/api/v1",
		"http://localhost:8890":   "http://localhost:8890/api/v1",
		"https://example.test/":   "https://example.test/api/v1",
		"https://example.test/x/": "https://example.test/x/api/v1",
	}
	for input, want := range tests {
		if got := serverAPIBaseURL(input); got != want {
			t.Fatalf("serverAPIBaseURL(%q) = %q, want %q", input, got, want)
		}
	}
}

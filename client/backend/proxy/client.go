package proxy

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zcopy-client-backend/auth"
	"zcopy-client-backend/utils"
)

type RemoteClient interface {
	RawRequest(method, path string, body io.Reader, contentType, token string) ([]byte, int, error)
	UploadFile(localFile, remoteDir, token string) error
	DownloadRemoteFile(remotePath, localPath, token string) error
	EnsureRemotePath(path, token string) error
}

type HTTPRemoteClient struct {
	HTTPc   *http.Client
	BaseURL string
}

func NewHTTPRemoteClient(httpc *http.Client, baseURL string) *HTTPRemoteClient {
	return &HTTPRemoteClient{HTTPc: httpc, BaseURL: baseURL}
}

func (c *HTTPRemoteClient) RawRequest(method, path string, body io.Reader, contentType, token string) ([]byte, int, error) {
	start := time.Now()
	var copiedBody []byte
	if body != nil {
		buf, _ := io.ReadAll(body)
		copiedBody = buf
	}
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequest(method, url, bytes.NewReader(copiedBody))
	if err != nil {
		slog.Error("RawRequest 请求创建失败", "method", method, "url", url, "error", err)
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTPc.Do(req)
	if err != nil {
		slog.Error("RawRequest 请求失败", "method", method, "url", url, "error", err)
		return nil, 0, err
	}
	defer resp.Body.Close()
	duration := time.Since(start)
	slog.Debug("RawRequest 请求完成", "method", method, "url", url, "status_code", resp.StatusCode, "duration_ms", duration.Milliseconds())
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

func (c *HTTPRemoteClient) UploadFile(localFile, remoteDir, token string) error {
	start := time.Now()
	f, err := os.Open(localFile)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

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

	_, status, err := c.RawRequest(http.MethodPost, "/files/upload", bytes.NewReader(body.Bytes()), writer.FormDataContentType(), token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return errors.New("上传文件失败")
	}
	duration := time.Since(start)
	slog.Info("文件上传完成", "filename", filepath.Base(localFile), "size", fi.Size(), "duration", duration)
	return nil
}

func (c *HTTPRemoteClient) DownloadRemoteFile(remotePath, localPath, token string) error {
	start := time.Now()
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/files/download?path=" + url.QueryEscape(utils.NormalizeRemote(remotePath))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.HTTPc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(resp.Body)
		msg := utils.ParseJSONMessage(buf)
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
	n, err := io.Copy(dst, resp.Body)
	if err == nil {
		duration := time.Since(start)
		slog.Info("文件下载完成", "filename", filepath.Base(localPath), "size", n, "duration", duration)
	}
	return err
}

func (c *HTTPRemoteClient) EnsureRemotePath(path, token string) error {
	clean := utils.NormalizeRemote(path)
	if clean == "" {
		return nil
	}
	slog.Debug("确保远程目录存在", "path", clean)
	parts := strings.Split(clean, "/")
	parent := ""
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		body, _ := json.Marshal(map[string]string{
			"path": parent,
			"name": p,
		})
		_, status, err := c.RawRequest(http.MethodPost, "/files/folder", bytes.NewReader(body), "application/json", token)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return fmt.Errorf("创建远程目录失败: %s", p)
		}
		if parent == "" {
			parent = p
		} else {
			parent += "/" + p
		}
	}
	return nil
}

var ErrServerNotRegistered = errors.New("server not registered")

type MultiServerProxy struct {
	clients      map[string]*HTTPRemoteClient
	tokenProvider auth.TokenProvider
	mu           sync.RWMutex
}

func NewMultiServerProxy(tokenProvider auth.TokenProvider) *MultiServerProxy {
	return &MultiServerProxy{
		clients:      make(map[string]*HTTPRemoteClient),
		tokenProvider: tokenProvider,
	}
}

func (p *MultiServerProxy) AddServer(serverID, baseURL string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients[serverID] = NewHTTPRemoteClient(http.DefaultClient, baseURL)
}

func (p *MultiServerProxy) RemoveServer(serverID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.clients, serverID)
}

func (p *MultiServerProxy) GetClient(serverID string) (*HTTPRemoteClient, error) {
	p.mu.RLock()
	client, exists := p.clients[serverID]
	p.mu.RUnlock()
	if !exists {
		return nil, ErrServerNotRegistered
	}
	return client, nil
}

func (p *MultiServerProxy) HasServer(serverID string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, exists := p.clients[serverID]
	return exists
}

func (p *MultiServerProxy) ListServerIDs() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.clients))
	for id := range p.clients {
		ids = append(ids, id)
	}
	return ids
}

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"
)

type fileProviderRegisterRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
}

type fileProviderBridgeStatus struct {
	Registered bool   `json:"registered"`
	Reason     string `json:"reason"`
	MountPath  string `json:"mountPath"`
	LogPath    string `json:"logPath"`
}

type remoteWebDAVItem struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	IsDirectory bool      `json:"isDirectory"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type remoteWebDAVListResponse struct {
	Path  string             `json:"path"`
	Items []remoteWebDAVItem `json:"items"`
}

type remoteWebDAVFS struct {
	app  *AppState
	task BackupTask
}

type remoteWebDAVInfo struct {
	name    string
	size    int64
	modTime time.Time
	mode    os.FileMode
}

type remoteWebDAVDirFile struct {
	info    os.FileInfo
	entries []os.FileInfo
	offset  int
}

type remoteWebDAVReadFile struct {
	*os.File
	cleanupPath string
}

type remoteWebDAVWriteFile struct {
	*os.File
	app         *AppState
	task        BackupTask
	relPath     string
	cleanupPath string
	closeOnce   sync.Once
	closeErr    error
}

func (a *AppState) startWebDAVServer() error {
	if runtime.GOOS != "darwin" {
		return nil
	}
	username := strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_WEBDAV_USER"))
	if username == "" {
		username = "zcopy"
	}
	password := strings.TrimSpace(os.Getenv("ZCOPY_CLIENT_WEBDAV_PASSWORD"))
	if password == "" {
		password = randomBridgeSecret(24)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	a.webdavBaseURL = "http://" + listener.Addr().String()
	a.webdavUsername = username
	a.webdavPassword = password

	server := &http.Server{
		Handler: http.HandlerFunc(a.serveWebDAV),
	}
	go func() {
		_ = server.Serve(listener)
	}()
	return nil
}

func (a *AppState) serveWebDAV(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/webdav/") {
		http.NotFound(w, r)
		return
	}
	user, password, ok := r.BasicAuth()
	if !ok || user != a.webdavUsername || password != a.webdavPassword {
		w.Header().Set("WWW-Authenticate", `Basic realm="ZCopy File Provider"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/webdav/")
	parts := strings.SplitN(trimmed, "/", 2)
	taskID := strings.TrimSpace(parts[0])
	if taskID == "" {
		http.NotFound(w, r)
		return
	}
	task, ok := a.store.get(taskID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler := &webdav.Handler{
		Prefix:     "/webdav/" + taskID,
		FileSystem: &remoteWebDAVFS{app: a, task: task},
		LockSystem: webdav.NewMemLS(),
	}
	handler.ServeHTTP(w, r)
}

func (a *AppState) fileProviderAvailable() bool {
	return runtime.GOOS == "darwin" && a.fpBridgeURL != "" && a.fpBridgeToken != "" && a.webdavBaseURL != ""
}

func (a *AppState) fileProviderTaskURL(taskID string) string {
	return strings.TrimRight(a.webdavBaseURL, "/") + "/webdav/" + url.PathEscape(taskID)
}

func (a *AppState) initTaskFileProvider(task BackupTask) (fileProviderBridgeStatus, error) {
	payload := fileProviderRegisterRequest{
		ID:       syncRootID(task.ID),
		Name:     "ZCopy " + task.Name,
		URL:      a.fileProviderTaskURL(task.ID),
		User:     a.webdavUsername,
		Password: a.webdavPassword,
	}
	var status fileProviderBridgeStatus
	if err := a.callFileProviderBridge(http.MethodPost, "/register", payload, &status); err != nil {
		return fileProviderBridgeStatus{}, err
	}
	return status, nil
}

func (a *AppState) getTaskFileProviderStatus(task BackupTask) (fileProviderBridgeStatus, error) {
	query := url.Values{}
	query.Set("id", syncRootID(task.ID))
	query.Set("name", "ZCopy "+task.Name)
	var status fileProviderBridgeStatus
	if err := a.callFileProviderBridge(http.MethodGet, "/status?"+query.Encode(), nil, &status); err != nil {
		return fileProviderBridgeStatus{}, err
	}
	return status, nil
}

func (a *AppState) callFileProviderBridge(method string, endpoint string, payload any, out any) error {
	if a.fpBridgeURL == "" || a.fpBridgeToken == "" {
		return errors.New("File Provider bridge 未配置")
	}
	var body io.Reader
	if payload != nil {
		buf, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
	}
	req, err := http.NewRequest(method, strings.TrimRight(a.fpBridgeURL, "/")+endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+a.fpBridgeToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure map[string]any
		if json.Unmarshal(data, &failure) == nil {
			if message, ok := failure["message"].(string); ok && message != "" {
				return errors.New(message)
			}
		}
		if len(data) == 0 {
			return errors.New("File Provider bridge 调用失败")
		}
		return errors.New(string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return err
		}
	}
	return nil
}

func (a *AppState) listRemoteItems(remotePath string, token string) ([]remoteWebDAVItem, error) {
	endpoint := "/files"
	clean := normalizeRemote(remotePath)
	if clean != "" {
		endpoint += "?path=" + url.QueryEscape(clean)
	}
	data, status, err := a.proxyRaw(http.MethodGet, endpoint, nil, "", token)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		msg := parseJSONMessage(data)
		if msg == "" {
			msg = "读取远程目录失败"
		}
		return nil, errors.New(msg)
	}
	var payload remoteWebDAVListResponse
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

func (a *AppState) deleteRemotePath(remotePath string, token string) error {
	endpoint := "/files?path=" + url.QueryEscape(normalizeRemote(remotePath))
	data, status, err := a.proxyRaw(http.MethodDelete, endpoint, nil, "", token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		msg := parseJSONMessage(data)
		if msg == "" {
			msg = "删除远程路径失败"
		}
		return errors.New(msg)
	}
	return nil
}

func (a *AppState) uploadFileReader(filename string, remoteDir string, src io.Reader, token string) error {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("path", normalizeRemote(remoteDir)); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, src); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	data, status, err := a.proxyRaw(http.MethodPost, "/files/upload", bytes.NewReader(body.Bytes()), writer.FormDataContentType(), token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		msg := parseJSONMessage(data)
		if msg == "" {
			msg = "上传文件失败"
		}
		return errors.New(msg)
	}
	return nil
}

func (a *AppState) remoteInfoForPath(task BackupTask, relPath string, token string) (os.FileInfo, error) {
	cleanRel := normalizeWebDAVPath(relPath)
	if cleanRel == "" {
		return remoteWebDAVInfo{
			name:    task.Name,
			size:    0,
			modTime: task.UpdatedAt,
			mode:    os.ModeDir | 0755,
		}, nil
	}
	parent := path.Dir(cleanRel)
	if parent == "." {
		parent = ""
	}
	base := path.Base(cleanRel)
	items, err := a.listRemoteItems(joinRemotePath(task.RemotePath, parent), token)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Name != base {
			continue
		}
		mode := os.FileMode(0644)
		if item.IsDirectory {
			mode = os.ModeDir | 0755
		}
		return remoteWebDAVInfo{
			name:    item.Name,
			size:    item.Size,
			modTime: item.UpdatedAt,
			mode:    mode,
		}, nil
	}
	return nil, os.ErrNotExist
}

func (a *AppState) remoteDirEntries(task BackupTask, relPath string, token string) ([]os.FileInfo, error) {
	items, err := a.listRemoteItems(joinRemotePath(task.RemotePath, relPath), token)
	if err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, 0, len(items))
	for _, item := range items {
		mode := os.FileMode(0644)
		if item.IsDirectory {
			mode = os.ModeDir | 0755
		}
		result = append(result, remoteWebDAVInfo{
			name:    item.Name,
			size:    item.Size,
			modTime: item.UpdatedAt,
			mode:    mode,
		})
	}
	return result, nil
}

func (fs *remoteWebDAVFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	token := fs.app.getToken()
	if token == "" {
		return os.ErrPermission
	}
	return fs.app.ensureRemotePath(joinRemotePath(fs.task.RemotePath, name), token)
}

func (fs *remoteWebDAVFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	cleanRel := normalizeWebDAVPath(name)
	token := fs.app.getToken()
	if token == "" {
		return nil, os.ErrPermission
	}

	writeMode := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0
	if !writeMode {
		info, err := fs.app.remoteInfoForPath(fs.task, cleanRel, token)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			entries, err := fs.app.remoteDirEntries(fs.task, cleanRel, token)
			if err != nil {
				return nil, err
			}
			return &remoteWebDAVDirFile{
				info:    info,
				entries: entries,
			}, nil
		}
		localPath := fs.app.webdavTempPath(fs.task.ID, cleanRel)
		if err := fs.app.downloadRemoteFile(joinRemotePath(fs.task.RemotePath, cleanRel), localPath, token); err != nil {
			return nil, err
		}
		file, err := os.Open(localPath)
		if err != nil {
			return nil, err
		}
		return &remoteWebDAVReadFile{
			File:        file,
			cleanupPath: localPath,
		}, nil
	}

	localPath := fs.app.webdavTempPath(fs.task.ID, cleanRel)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return nil, err
	}
	if flag&os.O_TRUNC == 0 {
		err := fs.app.downloadRemoteFile(joinRemotePath(fs.task.RemotePath, cleanRel), localPath, token)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	openFlags := os.O_CREATE | os.O_RDWR
	if flag&os.O_TRUNC != 0 {
		openFlags |= os.O_TRUNC
	}
	tempFile, err := os.OpenFile(localPath, openFlags, 0644)
	if err != nil {
		return nil, err
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		_ = tempFile.Close()
		return nil, err
	}
	if flag&os.O_APPEND != 0 {
		if _, err := tempFile.Seek(0, io.SeekEnd); err != nil {
			_ = tempFile.Close()
			return nil, err
		}
	}
	return &remoteWebDAVWriteFile{
		File:        tempFile,
		app:         fs.app,
		task:        fs.task,
		relPath:     cleanRel,
		cleanupPath: localPath,
	}, nil
}

func (fs *remoteWebDAVFS) RemoveAll(ctx context.Context, name string) error {
	token := fs.app.getToken()
	if token == "" {
		return os.ErrPermission
	}
	return fs.app.deleteRemotePath(joinRemotePath(fs.task.RemotePath, name), token)
}

func (fs *remoteWebDAVFS) Rename(ctx context.Context, oldName string, newName string) error {
	return errors.New("rename not supported")
}

func (fs *remoteWebDAVFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	token := fs.app.getToken()
	if token == "" {
		return nil, os.ErrPermission
	}
	return fs.app.remoteInfoForPath(fs.task, name, token)
}

func (info remoteWebDAVInfo) Name() string {
	return info.name
}

func (info remoteWebDAVInfo) Size() int64 {
	return info.size
}

func (info remoteWebDAVInfo) Mode() os.FileMode {
	return info.mode
}

func (info remoteWebDAVInfo) ModTime() time.Time {
	return info.modTime
}

func (info remoteWebDAVInfo) IsDir() bool {
	return info.mode.IsDir()
}

func (info remoteWebDAVInfo) Sys() any {
	return nil
}

func (f *remoteWebDAVDirFile) Close() error {
	return nil
}

func (f *remoteWebDAVDirFile) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (f *remoteWebDAVDirFile) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}

func (f *remoteWebDAVDirFile) Readdir(count int) ([]os.FileInfo, error) {
	if f.offset >= len(f.entries) {
		return nil, io.EOF
	}
	if count <= 0 {
		result := f.entries[f.offset:]
		f.offset = len(f.entries)
		return result, nil
	}
	end := f.offset + count
	if end > len(f.entries) {
		end = len(f.entries)
	}
	result := f.entries[f.offset:end]
	f.offset = end
	return result, nil
}

func (f *remoteWebDAVDirFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

func (f *remoteWebDAVDirFile) Write(p []byte) (int, error) {
	return 0, errors.New("not writable")
}

func (f *remoteWebDAVReadFile) Close() error {
	err := f.File.Close()
	_ = os.Remove(f.cleanupPath)
	return err
}

func (f *remoteWebDAVWriteFile) Close() error {
	f.closeOnce.Do(func() {
		if _, err := f.File.Seek(0, io.SeekStart); err != nil {
			f.closeErr = err
		} else {
			token := f.app.getToken()
			if token == "" {
				f.closeErr = os.ErrPermission
			} else {
				remotePath := joinRemotePath(f.task.RemotePath, f.relPath)
				remoteDir := path.Dir(remotePath)
				if remoteDir == "." {
					remoteDir = ""
				}
				if err := f.app.ensureRemotePath(remoteDir, token); err != nil {
					f.closeErr = err
				} else {
					_ = f.app.deleteRemotePath(remotePath, token)
					f.closeErr = f.app.uploadFileReader(path.Base(remotePath), remoteDir, f.File, token)
				}
			}
		}
		closeErr := f.File.Close()
		if f.closeErr == nil {
			f.closeErr = closeErr
		}
		_ = os.Remove(f.cleanupPath)
	})
	return f.closeErr
}

func (f *remoteWebDAVWriteFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, errors.New("not a directory")
}

func (a *AppState) webdavTempPath(taskID string, relPath string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(normalizeWebDAVPath(relPath)))
	return filepath.Join(a.cfg.Storage.DataDir, "webdav-cache", taskID, encoded)
}

func normalizeWebDAVPath(name string) string {
	clean := path.Clean("/" + filepath.ToSlash(strings.TrimSpace(name)))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "." {
		return ""
	}
	return clean
}

func joinRemotePath(base string, extra string) string {
	baseClean := normalizeRemote(base)
	extraClean := normalizeWebDAVPath(extra)
	switch {
	case baseClean == "":
		return extraClean
	case extraClean == "":
		return baseClean
	default:
		return normalizeRemote(baseClean + "/" + extraClean)
	}
}

func randomBridgeSecret(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

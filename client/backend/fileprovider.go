package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

	"github.com/gin-gonic/gin"
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

type fileProviderItemPayload struct {
	Identifier       string    `json:"identifier"`
	ParentIdentifier string    `json:"parentIdentifier"`
	Name             string    `json:"name"`
	Path             string    `json:"path"`
	Size             int64     `json:"size"`
	IsDirectory      bool      `json:"isDirectory"`
	UpdatedAt        time.Time `json:"updatedAt"`
	ChildCount       int       `json:"childCount"`
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

func (a *AppState) renameRemotePath(oldPath string, newPath string, token string) error {
	body, err := json.Marshal(gin.H{
		"from": normalizeRemote(oldPath),
		"to":   normalizeRemote(newPath),
	})
	if err != nil {
		return err
	}
	data, status, err := a.proxyRaw(http.MethodPut, "/files/rename", bytes.NewReader(body), "application/json", token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = fmt.Sprintf("rename failed with status %d", status)
		}
		return errors.New(message)
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

func taskLocalPath(task BackupTask, relPath string) string {
	clean := normalizeWebDAVPath(relPath)
	if clean == "" {
		return task.LocalPath
	}
	return filepath.Join(task.LocalPath, filepath.FromSlash(clean))
}

func localInfoForPath(task BackupTask, relPath string) (os.FileInfo, error) {
	return os.Stat(taskLocalPath(task, relPath))
}

func ensureLocalParent(task BackupTask, relPath string) error {
	return os.MkdirAll(filepath.Dir(taskLocalPath(task, relPath)), 0755)
}

func writeLocalMirrorFile(task BackupTask, relPath string, src io.Reader) error {
	localPath := taskLocalPath(task, relPath)
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return err
	}
	dst, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func renameLocalMirrorPath(task BackupTask, oldRelPath string, newRelPath string) error {
	oldPath := taskLocalPath(task, oldRelPath)
	newPath := taskLocalPath(task, newRelPath)
	if oldPath == newPath {
		return nil
	}
	if _, err := os.Stat(oldPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
		return err
	}
	return os.Rename(oldPath, newPath)
}

func (fs *remoteWebDAVFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	token := fs.app.getToken()
	if token == "" {
		return os.ErrPermission
	}
	if err := fs.app.ensureRemotePath(joinRemotePath(fs.task.RemotePath, name), token); err != nil {
		return err
	}
	return os.MkdirAll(taskLocalPath(fs.task, name), 0755)
}

func (fs *remoteWebDAVFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	cleanRel := normalizeWebDAVPath(name)
	token := fs.app.getToken()
	if token == "" {
		return nil, os.ErrPermission
	}

	writeMode := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0
	if !writeMode {
		if info, err := localInfoForPath(fs.task, cleanRel); err == nil {
			if info.IsDir() {
				entries, readErr := os.ReadDir(taskLocalPath(fs.task, cleanRel))
				if readErr != nil {
					return nil, readErr
				}
				dirEntries := make([]os.FileInfo, 0, len(entries))
				for _, entry := range entries {
					info, infoErr := entry.Info()
					if infoErr == nil {
						dirEntries = append(dirEntries, info)
					}
				}
				return &remoteWebDAVDirFile{info: info, entries: dirEntries}, nil
			}
			file, openErr := os.Open(taskLocalPath(fs.task, cleanRel))
			if openErr == nil {
				return &remoteWebDAVReadFile{File: file}, nil
			}
		}
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
		sourceLocalPath := taskLocalPath(fs.task, cleanRel)
		if _, err := os.Stat(sourceLocalPath); err == nil {
			src, openErr := os.Open(sourceLocalPath)
			if openErr != nil {
				return nil, openErr
			}
			dst, createErr := os.Create(localPath)
			if createErr != nil {
				_ = src.Close()
				return nil, createErr
			}
			_, copyErr := io.Copy(dst, src)
			_ = dst.Close()
			_ = src.Close()
			if copyErr != nil {
				return nil, copyErr
			}
		} else {
			err := fs.app.downloadRemoteFile(joinRemotePath(fs.task.RemotePath, cleanRel), localPath, token)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
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
	if err := fs.app.deleteRemotePath(joinRemotePath(fs.task.RemotePath, name), token); err != nil {
		return err
	}
	_ = os.RemoveAll(taskLocalPath(fs.task, name))
	return nil
}

func (fs *remoteWebDAVFS) Rename(ctx context.Context, oldName string, newName string) error {
	token := fs.app.getToken()
	if token == "" {
		return os.ErrPermission
	}
	if err := fs.app.renameRemotePath(joinRemotePath(fs.task.RemotePath, oldName), joinRemotePath(fs.task.RemotePath, newName), token); err != nil {
		return err
	}
	return renameLocalMirrorPath(fs.task, oldName, newName)
}

func (fs *remoteWebDAVFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	token := fs.app.getToken()
	if token == "" {
		return nil, os.ErrPermission
	}
	if info, err := localInfoForPath(fs.task, name); err == nil {
		return info, nil
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
	if f.cleanupPath != "" {
		_ = os.Remove(f.cleanupPath)
	}
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
					if f.closeErr == nil {
						if _, err := f.File.Seek(0, io.SeekStart); err != nil {
							f.closeErr = err
						} else {
							f.closeErr = writeLocalMirrorFile(f.task, f.relPath, f.File)
						}
					}
				}
			}
		}
		closeErr := f.File.Close()
		if f.closeErr == nil {
			f.closeErr = closeErr
		}
		if f.cleanupPath != "" {
			_ = os.Remove(f.cleanupPath)
		}
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

func (a *AppState) fileProviderItem(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	info, err := a.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"message": "远程项目不存在"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程项目失败: " + err.Error()})
		return
	}
	payload, err := a.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程项目失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (a *AppState) fileProviderChildren(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	items, err := a.listRemoteItems(joinRemotePath(task.RemotePath, cleanPath), token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程目录失败: " + err.Error()})
		return
	}
	payload := make([]fileProviderItemPayload, 0, len(items))
	for _, item := range items {
		childPath := normalizeWebDAVPath(path.Join(cleanPath, item.Name))
		mode := os.FileMode(0644)
		if item.IsDirectory {
			mode = os.ModeDir | 0755
		}
		entry, err := a.makeFileProviderItemPayload(task, childPath, remoteWebDAVInfo{
			name:    item.Name,
			size:    item.Size,
			modTime: item.UpdatedAt,
			mode:    mode,
		}, token)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程目录项失败: " + err.Error()})
			return
		}
		payload = append(payload, entry)
	}
	c.JSON(http.StatusOK, gin.H{
		"path":  cleanPath,
		"items": payload,
	})
}

func (a *AppState) fileProviderContent(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "根目录没有可下载内容"})
		return
	}
	if info, err := localInfoForPath(task, cleanPath); err == nil && !info.IsDir() {
		c.File(taskLocalPath(task, cleanPath))
		return
	}
	info, err := a.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"message": "远程项目不存在"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程项目失败: " + err.Error()})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"message": "目录不能直接下载"})
		return
	}
	endpoint := strings.TrimRight(a.cfg.FileServer.BaseURL, "/") + "/files/download?path=" + url.QueryEscape(joinRemotePath(task.RemotePath, cleanPath))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.httpc.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "下载远程文件失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		msg := parseJSONMessage(data)
		if msg == "" {
			msg = "下载远程文件失败"
		}
		c.JSON(http.StatusBadGateway, gin.H{"message": msg})
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", `attachment; filename="`+path.Base(cleanPath)+`"`)
	c.Status(http.StatusOK)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		c.Error(err)
	}
}

func (a *AppState) fileProviderPutContent(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "根目录不能直接写入"})
		return
	}
	remotePath := joinRemotePath(task.RemotePath, cleanPath)
	remoteDir := path.Dir(remotePath)
	if remoteDir == "." {
		remoteDir = ""
	}
	if err := a.ensureRemotePath(remoteDir, token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "确保远程目录失败: " + err.Error()})
		return
	}
	tempFile, err := os.CreateTemp("", "zcopy-fileprovider-put-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建临时文件失败: " + err.Error()})
		return
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()
	if _, err := io.Copy(tempFile, c.Request.Body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "读取上传内容失败: " + err.Error()})
		return
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "重置临时文件失败: " + err.Error()})
		return
	}
	_ = a.deleteRemotePath(remotePath, token)
	if err := a.uploadFileReader(path.Base(cleanPath), remoteDir, tempFile, token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "上传远程文件失败: " + err.Error()})
		return
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "重置本地镜像文件失败: " + err.Error()})
		return
	}
	if err := writeLocalMirrorFile(task, cleanPath, tempFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "更新本地文件失败: " + err.Error()})
		return
	}
	info, err := a.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取上传结果失败: " + err.Error()})
		return
	}
	payload, err := a.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换上传结果失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (a *AppState) fileProviderRenameItem(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能重命名根目录"})
		return
	}
	var req struct {
		NewPath string `json:"newPath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}
	newPath := normalizeWebDAVPath(req.NewPath)
	if newPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "目标路径不合法"})
		return
	}
	if err := a.renameRemotePath(joinRemotePath(task.RemotePath, cleanPath), joinRemotePath(task.RemotePath, newPath), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "重命名远程项目失败: " + err.Error()})
		return
	}
	if err := renameLocalMirrorPath(task, cleanPath, newPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "重命名本地项目失败: " + err.Error()})
		return
	}
	info, err := a.remoteInfoForPath(task, newPath, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取重命名结果失败: " + err.Error()})
		return
	}
	payload, err := a.makeFileProviderItemPayload(task, newPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换重命名结果失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (a *AppState) fileProviderCreateFolder(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	if err := a.ensureRemotePath(joinRemotePath(task.RemotePath, cleanPath), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "创建远程目录失败: " + err.Error()})
		return
	}
	if err := os.MkdirAll(taskLocalPath(task, cleanPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建本地目录失败: " + err.Error()})
		return
	}
	info, err := a.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程目录失败: " + err.Error()})
		return
	}
	payload, err := a.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程目录失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (a *AppState) fileProviderDeleteItem(c *gin.Context) {
	task, cleanPath, token, ok := a.fileProviderContext(c)
	if !ok {
		return
	}
	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能删除根目录"})
		return
	}
	if err := a.deleteRemotePath(joinRemotePath(task.RemotePath, cleanPath), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "删除远程项目失败: " + err.Error()})
		return
	}
	_ = os.RemoveAll(taskLocalPath(task, cleanPath))
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (a *AppState) fileProviderContext(c *gin.Context) (BackupTask, string, string, bool) {
	taskID := normalizeFileProviderTaskID(c.Param("id"))
	task, ok := a.store.get(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return BackupTask{}, "", "", false
	}
	token := a.getToken()
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return BackupTask{}, "", "", false
	}
	return task, normalizeWebDAVPath(c.Query("path")), token, true
}

func normalizeFileProviderTaskID(raw string) string {
	taskID := strings.TrimSpace(raw)
	taskID = strings.TrimPrefix(taskID, "ZCopy.")
	if idx := strings.Index(taskID, "task-"); idx >= 0 {
		return taskID[idx:]
	}
	return taskID
}

func (a *AppState) makeFileProviderItemPayload(task BackupTask, cleanPath string, info os.FileInfo, token string) (fileProviderItemPayload, error) {
	payload := fileProviderItemPayload{
		Identifier:       fileProviderIdentifier(cleanPath),
		ParentIdentifier: fileProviderIdentifier(path.Dir(cleanPath)),
		Name:             info.Name(),
		Path:             cleanPath,
		Size:             info.Size(),
		IsDirectory:      info.IsDir(),
		UpdatedAt:        info.ModTime(),
	}
	if cleanPath == "" {
		payload.ParentIdentifier = "root"
		payload.Name = task.Name
	}
	if payload.IsDirectory {
		children, err := a.remoteDirEntries(task, cleanPath, token)
		if err != nil {
			return fileProviderItemPayload{}, err
		}
		payload.ChildCount = len(children)
	}
	return payload, nil
}

func fileProviderIdentifier(cleanPath string) string {
	if normalizeWebDAVPath(cleanPath) == "" {
		return "root"
	}
	return "path:" + normalizeWebDAVPath(cleanPath)
}

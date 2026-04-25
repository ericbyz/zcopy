package fileprovider

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"zcopy-client-backend/auth"
	logpkg "zcopy-client-backend/log"
	"zcopy-client-backend/models"
	"zcopy-client-backend/platform"
	"zcopy-client-backend/proxy"
	"zcopy-client-backend/store"
	"zcopy-client-backend/utils"
)

type Service struct {
	cfg           models.AppConfig
	httpc         *http.Client
	store         store.TaskRepository
	tokens        auth.TokenManager
	remote        proxy.RemoteClient
	logs          logpkg.LogStore
	fpBridgeURL   string
	fpBridgeToken string

	webdavBaseURL  string
	webdavUsername string
	webdavPassword string
}

type BridgeStatus struct {
	Registered bool   `json:"registered"`
	Reason     string `json:"reason"`
	MountPath  string `json:"mountPath"`
	LogPath    string `json:"logPath"`
}

type fileProviderRegisterRequest struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	User     string `json:"user"`
	Password string `json:"password"`
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

func New(cfg models.AppConfig, httpc *http.Client, taskStore store.TaskRepository, tokens auth.TokenManager, remote proxy.RemoteClient, logs logpkg.LogStore, bridgeURL string, bridgeToken string) *Service {
	return &Service{
		cfg:           cfg,
		httpc:         httpc,
		store:         taskStore,
		tokens:        tokens,
		remote:        remote,
		logs:          logs,
		fpBridgeURL:   strings.TrimSpace(bridgeURL),
		fpBridgeToken: strings.TrimSpace(bridgeToken),
	}
}

func (s *Service) StartWebDAVServer() error {
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
	s.webdavBaseURL = "http://" + listener.Addr().String()
	s.webdavUsername = username
	s.webdavPassword = password
	slog.Info("WebDAV 服务已启动", "url", s.webdavBaseURL)

	server := &http.Server{Handler: http.HandlerFunc(s.serveWebDAV)}
	go func() {
		_ = server.Serve(listener)
	}()
	return nil
}

func (s *Service) Available() bool {
	return runtime.GOOS == "darwin" && s.fpBridgeURL != "" && s.fpBridgeToken != "" && s.webdavBaseURL != ""
}

func (s *Service) InitTask(task models.BackupTask) (BridgeStatus, error) {
	slog.Info("[FP] 域注册开始", "task_id", task.ID, "task_name", task.Name)
	if s.logs != nil {
		s.logs.Push("info", task, "", fmt.Sprintf("[FP] 域注册开始：task_id=%s, task_name=%s", task.ID, task.Name))
	}

	payload := fileProviderRegisterRequest{
		ID:       platform.SyncRootID(task.ID),
		Name:     platform.CloudFolderDisplayName(task.Name),
		URL:      s.fileProviderTaskURL(task.ID),
		User:     s.webdavUsername,
		Password: s.webdavPassword,
	}
	var status BridgeStatus
	if err := s.callFileProviderBridge(http.MethodPost, "/register", payload, &status); err != nil {
		slog.Error("[FP] 域注册失败", "task_id", task.ID, "error", err)
		if s.logs != nil {
			s.logs.Push("error", task, "", fmt.Sprintf("[FP] 域注册失败：error=%s", err.Error()))
		}
		return BridgeStatus{}, err
	}

	slog.Info("[FP] 域注册完成", "task_id", task.ID, "registered", status.Registered, "mount_path", status.MountPath)
	if s.logs != nil {
		s.logs.Push("info", task, "", fmt.Sprintf("[FP] 域注册完成：registered=%v, mount_path=%s", status.Registered, status.MountPath))
	}
	return status, nil
}

func (s *Service) GetTaskStatus(task models.BackupTask) (BridgeStatus, error) {
	slog.Debug("[FP] 查询域状态", "task_id", task.ID, "task_name", task.Name)

	query := url.Values{}
	query.Set("id", platform.SyncRootID(task.ID))
	query.Set("name", platform.CloudFolderDisplayName(task.Name))
	var status BridgeStatus
	if err := s.callFileProviderBridge(http.MethodGet, "/status?"+query.Encode(), nil, &status); err != nil {
		slog.Error("[FP] 查询域状态失败", "task_id", task.ID, "error", err)
		if s.logs != nil {
			s.logs.Push("error", task, "", fmt.Sprintf("[FP] 查询域状态失败：error=%s", err.Error()))
		}
		return BridgeStatus{}, err
	}

	slog.Debug("[FP] 查询域状态完成", "task_id", task.ID, "registered", status.Registered, "mount_path", status.MountPath)
	return status, nil
}

func (s *Service) fileProviderTaskURL(taskID string) string {
	return strings.TrimRight(s.webdavBaseURL, "/") + "/webdav/" + url.PathEscape(taskID)
}

func (s *Service) callFileProviderBridge(method string, endpoint string, payload any, out any) error {
	if s.fpBridgeURL == "" || s.fpBridgeToken == "" {
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
	url := strings.TrimRight(s.fpBridgeURL, "/") + endpoint
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		slog.Error("File Provider bridge 请求创建失败", "url", url, "error", err)
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.fpBridgeToken)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.httpc.Do(req)
	if err != nil {
		slog.Error("File Provider bridge 请求失败", "url", url, "error", err)
		return err
	}
	defer resp.Body.Close()

	slog.Debug("File Provider bridge 调用", "url", url, "status_code", resp.StatusCode)

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

func (s *Service) listRemoteItems(remotePath string, token string) ([]remoteWebDAVItem, error) {
	slog.Debug("listRemoteItems 操作", "remote_path", remotePath)
	endpoint := "/files"
	clean := utils.NormalizeRemote(remotePath)
	if clean != "" {
		endpoint += "?path=" + url.QueryEscape(clean)
	}
	data, status, err := s.remote.RawRequest(http.MethodGet, endpoint, nil, "", token)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		msg := utils.ParseJSONMessage(data)
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

func (s *Service) deleteRemotePath(remotePath string, token string) error {
	slog.Debug("deleteRemotePath 操作", "remote_path", remotePath)
	endpoint := "/files?path=" + url.QueryEscape(utils.NormalizeRemote(remotePath))
	data, status, err := s.remote.RawRequest(http.MethodDelete, endpoint, nil, "", token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		msg := utils.ParseJSONMessage(data)
		if msg == "" {
			msg = "删除远程路径失败"
		}
		return errors.New(msg)
	}
	return nil
}

func (s *Service) renameRemotePath(oldPath string, newPath string, token string) error {
	slog.Debug("renameRemotePath 操作", "old_path", oldPath, "new_path", newPath)
	body, err := json.Marshal(map[string]string{
		"from": utils.NormalizeRemote(oldPath),
		"to":   utils.NormalizeRemote(newPath),
	})
	if err != nil {
		return err
	}
	data, status, err := s.remote.RawRequest(http.MethodPut, "/files/rename", bytes.NewReader(body), "application/json", token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		message := strings.TrimSpace(string(data))
		if message == "" {
			message = "rename failed"
		}
		return errors.New(message)
	}
	return nil
}

func (s *Service) uploadFileReader(filename string, remoteDir string, src io.Reader, token string) error {
	slog.Debug("uploadFileReader 操作", "filename", filename, "remote_dir", remoteDir)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("path", utils.NormalizeRemote(remoteDir)); err != nil {
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

	data, status, err := s.remote.RawRequest(http.MethodPost, "/files/upload", bytes.NewReader(body.Bytes()), writer.FormDataContentType(), token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		msg := utils.ParseJSONMessage(data)
		if msg == "" {
			msg = "上传文件失败"
		}
		return errors.New(msg)
	}
	return nil
}

func (s *Service) remoteInfoForPath(task models.BackupTask, relPath string, token string) (os.FileInfo, error) {
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
	items, err := s.listRemoteItems(joinRemotePath(task.RemotePath, parent), token)
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

func (s *Service) remoteDirEntries(task models.BackupTask, relPath string, token string) ([]os.FileInfo, error) {
	items, err := s.listRemoteItems(joinRemotePath(task.RemotePath, relPath), token)
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

func (s *Service) fileProviderContext(c *gin.Context) (models.BackupTask, string, string, bool) {
	taskID := normalizeFileProviderTaskID(c.Param("id"))
	task, ok := s.store.Get(taskID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"message": "任务不存在"})
		return models.BackupTask{}, "", "", false
	}
	token := s.tokens.GetToken()
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return models.BackupTask{}, "", "", false
	}
	return task, normalizeWebDAVPath(c.Query("path")), token, true
}

func (s *Service) makeFileProviderItemPayload(task models.BackupTask, cleanPath string, info os.FileInfo, token string) (fileProviderItemPayload, error) {
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
		children, err := s.remoteDirEntries(task, cleanPath, token)
		if err != nil {
			return fileProviderItemPayload{}, err
		}
		payload.ChildCount = len(children)
	}
	return payload, nil
}

func taskLocalPath(task models.BackupTask, relPath string) string {
	clean := normalizeWebDAVPath(relPath)
	if clean == "" {
		return task.LocalPath
	}
	return filepath.Join(task.LocalPath, filepath.FromSlash(clean))
}

func localInfoForPath(task models.BackupTask, relPath string) (os.FileInfo, error) {
	return os.Stat(taskLocalPath(task, relPath))
}

func writeLocalMirrorFile(task models.BackupTask, relPath string, src io.Reader) error {
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

func renameLocalMirrorPath(task models.BackupTask, oldRelPath string, newRelPath string) error {
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

func (s *Service) webdavTempPath(taskID string, relPath string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(normalizeWebDAVPath(relPath)))
	return filepath.Join(s.cfg.Storage.DataDir, "webdav-cache", taskID, encoded)
}

func normalizeFileProviderTaskID(raw string) string {
	taskID := strings.TrimSpace(raw)
	taskID = strings.TrimPrefix(taskID, "ZCopy.")
	if idx := strings.Index(taskID, "task-"); idx >= 0 {
		return taskID[idx:]
	}
	return taskID
}

func fileProviderIdentifier(cleanPath string) string {
	if normalizeWebDAVPath(cleanPath) == "" {
		return "root"
	}
	return "path:" + normalizeWebDAVPath(cleanPath)
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
	baseClean := utils.NormalizeRemote(base)
	extraClean := normalizeWebDAVPath(extra)
	switch {
	case baseClean == "":
		return extraClean
	case extraClean == "":
		return baseClean
	default:
		return utils.NormalizeRemote(baseClean + "/" + extraClean)
	}
}

func randomBridgeSecret(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

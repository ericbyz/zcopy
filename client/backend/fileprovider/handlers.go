package fileprovider

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"zcopy-client-backend/models"
	"zcopy-client-backend/utils"
)

func (s *Service) fpLog(level, filePath, message string, task models.BackupTask) {
	if s.logs == nil {
		return
	}
	s.logs.Push(level, task, filePath, message)
}

func (s *Service) Item(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 获取项目元数据", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 获取项目元数据：path=%s", cleanPath), task)

	info, err := s.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("[FP] 远程项目不存在", "task_id", task.ID, "path", cleanPath)
			s.fpLog("warn", cleanPath, fmt.Sprintf("[FP] 远程项目不存在：path=%s", cleanPath), task)
			c.JSON(http.StatusNotFound, gin.H{"message": "远程项目不存在"})
			return
		}
		slog.Error("[FP] 读取远程项目失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 读取远程项目失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程项目失败: " + err.Error()})
		return
	}
	payload, err := s.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		slog.Error("[FP] 转换远程项目失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 转换远程项目失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程项目失败: " + err.Error()})
		return
	}

	slog.Info("[FP] 获取项目元数据完成", "task_id", task.ID, "path", cleanPath, "name", payload.Name, "is_dir", payload.IsDirectory)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 获取项目元数据完成：name=%s, is_dir=%v, size=%d", payload.Name, payload.IsDirectory, payload.Size), task)
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) Children(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 列出目录子项", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 列出目录子项：path=%s", cleanPath), task)

	items, err := s.listRemoteItems(joinRemotePath(task.RemotePath, cleanPath), token)
	if err != nil {
		slog.Error("[FP] 读取远程目录失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 读取远程目录失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程目录失败: " + err.Error()})
		return
	}
	payload := make([]fileProviderItemPayload, 0, len(items))
	for _, item := range items {
		childPath := normalizeFileProviderPath(path.Join(cleanPath, item.Name))
		mode := os.FileMode(0644)
		if item.IsDirectory {
			mode = os.ModeDir | 0755
		}
		entry, err := s.makeFileProviderItemPayload(task, childPath, remoteFileProviderInfo{
			name:    item.Name,
			size:    item.Size,
			modTime: item.UpdatedAt,
			mode:    mode,
		}, token)
		if err != nil {
			slog.Error("[FP] 转换远程目录项失败", "task_id", task.ID, "path", childPath, "error", err)
			s.fpLog("error", childPath, fmt.Sprintf("[FP] 转换远程目录项失败：path=%s, error=%s", childPath, err.Error()), task)
			c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程目录项失败: " + err.Error()})
			return
		}
		payload = append(payload, entry)
	}

	var names []string
	for _, p := range payload {
		names = append(names, p.Name)
	}
	slog.Info("[FP] 列出目录子项完成", "task_id", task.ID, "path", cleanPath, "count", len(payload))
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 列出目录子项完成：path=%s, count=%d, items=%s", cleanPath, len(payload), strings.Join(names, ", ")), task)
	c.JSON(http.StatusOK, gin.H{"path": cleanPath, "items": payload})
}

func (s *Service) Content(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 下载文件内容", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 下载文件内容：path=%s", cleanPath), task)

	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "根目录没有可下载内容"})
		return
	}
	if info, err := localInfoForPath(task, cleanPath); err == nil && !info.IsDir() {
		slog.Info("[FP] 命中本地缓存，直接返回本地文件", "task_id", task.ID, "path", cleanPath)
		s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 下载命中本地缓存：path=%s", cleanPath), task)
		c.File(taskLocalPath(task, cleanPath))
		return
	}
	info, err := s.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			slog.Warn("[FP] 远程项目不存在", "task_id", task.ID, "path", cleanPath)
			s.fpLog("warn", cleanPath, fmt.Sprintf("[FP] 下载失败，远程项目不存在：path=%s", cleanPath), task)
			c.JSON(http.StatusNotFound, gin.H{"message": "远程项目不存在"})
			return
		}
		slog.Error("[FP] 读取远程项目失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 下载失败，读取远程项目失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程项目失败: " + err.Error()})
		return
	}
	if info.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"message": "目录不能直接下载"})
		return
	}
	endpoint := strings.TrimRight(s.cfg.FileServer.BaseURL, "/") + "/files/download?path=" + url.QueryEscape(joinRemotePath(task.RemotePath, cleanPath))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpc.Do(req)
	if err != nil {
		slog.Error("[FP] 下载远程文件失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 下载远程文件失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "下载远程文件失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		msg := utils.ParseJSONMessage(data)
		if msg == "" {
			msg = "下载远程文件失败"
		}
		slog.Error("[FP] 下载远程文件失败", "task_id", task.ID, "path", cleanPath, "status_code", resp.StatusCode)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 下载远程文件失败：path=%s, status_code=%d, error=%s", cleanPath, resp.StatusCode, msg), task)
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

	slog.Info("[FP] 下载远程文件完成", "task_id", task.ID, "path", cleanPath, "size", info.Size())
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 下载远程文件完成：path=%s, size=%d", cleanPath, info.Size()), task)

	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		c.Error(err)
	}
}

func (s *Service) PutContent(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 上传文件内容", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 上传文件内容：path=%s", cleanPath), task)

	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "根目录不能直接写入"})
		return
	}
	remotePath := joinRemotePath(task.RemotePath, cleanPath)
	remoteDir := path.Dir(remotePath)
	if remoteDir == "." {
		remoteDir = ""
	}
	if err := s.remote.EnsureRemotePath(remoteDir, token); err != nil {
		slog.Error("[FP] 确保远程目录失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 上传失败，确保远程目录失败：path=%s, error=%s", cleanPath, err.Error()), task)
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
	_ = s.deleteRemotePath(remotePath, token)
	if err := s.uploadFileReader(path.Base(cleanPath), remoteDir, tempFile, token); err != nil {
		slog.Error("[FP] 上传远程文件失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 上传远程文件失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "上传远程文件失败: " + err.Error()})
		return
	}
	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "重置本地镜像文件失败: " + err.Error()})
		return
	}
	if err := writeLocalMirrorFile(task, cleanPath, tempFile); err != nil {
		slog.Error("[FP] 更新本地文件失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 上传成功但更新本地镜像失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "更新本地文件失败: " + err.Error()})
		return
	}
	info, err := s.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取上传结果失败: " + err.Error()})
		return
	}
	payload, err := s.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换上传结果失败: " + err.Error()})
		return
	}

	slog.Info("[FP] 上传文件完成", "task_id", task.ID, "path", cleanPath, "size", payload.Size)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 上传文件完成：path=%s, size=%d", cleanPath, payload.Size), task)
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) RenameItem(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 重命名项目", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 重命名项目：path=%s", cleanPath), task)

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
	newPath := normalizeFileProviderPath(req.NewPath)
	if newPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "目标路径不合法"})
		return
	}

	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 重命名项目：from=%s, to=%s", cleanPath, newPath), task)

	if err := s.renameRemotePath(joinRemotePath(task.RemotePath, cleanPath), joinRemotePath(task.RemotePath, newPath), token); err != nil {
		slog.Error("[FP] 重命名远程项目失败", "task_id", task.ID, "from", cleanPath, "to", newPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 重命名远程项目失败：from=%s, to=%s, error=%s", cleanPath, newPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "重命名远程项目失败: " + err.Error()})
		return
	}
	if err := renameLocalMirrorPath(task, cleanPath, newPath); err != nil {
		slog.Error("[FP] 重命名本地项目失败", "task_id", task.ID, "from", cleanPath, "to", newPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 重命名远程成功但本地失败：from=%s, to=%s, error=%s", cleanPath, newPath, err.Error()), task)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "重命名本地项目失败: " + err.Error()})
		return
	}
	info, err := s.remoteInfoForPath(task, newPath, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取重命名结果失败: " + err.Error()})
		return
	}
	payload, err := s.makeFileProviderItemPayload(task, newPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换重命名结果失败: " + err.Error()})
		return
	}

	slog.Info("[FP] 重命名完成", "task_id", task.ID, "from", cleanPath, "to", newPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 重命名完成：from=%s, to=%s", cleanPath, newPath), task)
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) CreateFolder(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 创建目录", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 创建目录：path=%s", cleanPath), task)

	if err := s.remote.EnsureRemotePath(joinRemotePath(task.RemotePath, cleanPath), token); err != nil {
		slog.Error("[FP] 创建远程目录失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 创建远程目录失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "创建远程目录失败: " + err.Error()})
		return
	}
	if err := os.MkdirAll(taskLocalPath(task, cleanPath), 0755); err != nil {
		slog.Error("[FP] 创建本地目录失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 创建本地目录失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建本地目录失败: " + err.Error()})
		return
	}
	info, err := s.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程目录失败: " + err.Error()})
		return
	}
	payload, err := s.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程目录失败: " + err.Error()})
		return
	}

	slog.Info("[FP] 创建目录完成", "task_id", task.ID, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 创建目录完成：path=%s", cleanPath), task)
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) DeleteItem(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}

	slog.Info("[FP] 删除项目", "task_id", task.ID, "task_name", task.Name, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 删除项目：path=%s", cleanPath), task)

	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能删除根目录"})
		return
	}
	if err := s.deleteRemotePath(joinRemotePath(task.RemotePath, cleanPath), token); err != nil {
		slog.Error("[FP] 删除远程项目失败", "task_id", task.ID, "path", cleanPath, "error", err)
		s.fpLog("error", cleanPath, fmt.Sprintf("[FP] 删除远程项目失败：path=%s, error=%s", cleanPath, err.Error()), task)
		c.JSON(http.StatusBadGateway, gin.H{"message": "删除远程项目失败: " + err.Error()})
		return
	}
	_ = os.RemoveAll(taskLocalPath(task, cleanPath))

	slog.Info("[FP] 删除项目完成", "task_id", task.ID, "path", cleanPath)
	s.fpLog("info", cleanPath, fmt.Sprintf("[FP] 删除项目完成：path=%s", cleanPath), task)
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

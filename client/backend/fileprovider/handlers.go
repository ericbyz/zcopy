package fileprovider

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"zcopy-client-backend/utils"
)

func (s *Service) Item(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}
	info, err := s.remoteInfoForPath(task, cleanPath, token)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.JSON(http.StatusNotFound, gin.H{"message": "远程项目不存在"})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"message": "读取远程项目失败: " + err.Error()})
		return
	}
	payload, err := s.makeFileProviderItemPayload(task, cleanPath, info, token)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "转换远程项目失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) Children(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}
	items, err := s.listRemoteItems(joinRemotePath(task.RemotePath, cleanPath), token)
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
		entry, err := s.makeFileProviderItemPayload(task, childPath, remoteWebDAVInfo{
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
	c.JSON(http.StatusOK, gin.H{"path": cleanPath, "items": payload})
}

func (s *Service) Content(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
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
	info, err := s.remoteInfoForPath(task, cleanPath, token)
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
	endpoint := strings.TrimRight(s.cfg.FileServer.BaseURL, "/") + "/files/download?path=" + url.QueryEscape(joinRemotePath(task.RemotePath, cleanPath))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.httpc.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "下载远程文件失败: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		msg := utils.ParseJSONMessage(data)
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

func (s *Service) PutContent(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
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
	if err := s.remote.EnsureRemotePath(remoteDir, token); err != nil {
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
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) RenameItem(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
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
	if err := s.renameRemotePath(joinRemotePath(task.RemotePath, cleanPath), joinRemotePath(task.RemotePath, newPath), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "重命名远程项目失败: " + err.Error()})
		return
	}
	if err := renameLocalMirrorPath(task, cleanPath, newPath); err != nil {
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
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) CreateFolder(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}
	if err := s.remote.EnsureRemotePath(joinRemotePath(task.RemotePath, cleanPath), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "创建远程目录失败: " + err.Error()})
		return
	}
	if err := os.MkdirAll(taskLocalPath(task, cleanPath), 0755); err != nil {
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
	c.JSON(http.StatusOK, gin.H{"item": payload})
}

func (s *Service) DeleteItem(c *gin.Context) {
	task, cleanPath, token, ok := s.fileProviderContext(c)
	if !ok {
		return
	}
	if cleanPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能删除根目录"})
		return
	}
	if err := s.deleteRemotePath(joinRemotePath(task.RemotePath, cleanPath), token); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"message": "删除远程项目失败: " + err.Error()})
		return
	}
	_ = os.RemoveAll(taskLocalPath(task, cleanPath))
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

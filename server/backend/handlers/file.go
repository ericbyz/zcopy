package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"zcopy-server-backend/config"
	"zcopy-server-backend/logger"
	"zcopy-server-backend/middleware"
	"zcopy-server-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createFolderRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type renameFileRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type fileItem struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	IsDirectory bool      `json:"isDirectory"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func ListFiles(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	relativePath, absolutePath, err := resolveUserPath(user.ID, c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}

	entries, err := os.ReadDir(absolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{
				"path":  relativePath,
				"items": []fileItem{},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"message": "读取目录失败"})
		return
	}

	items := make([]fileItem, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		itemPath := filepath.ToSlash(filepath.Join(relativePath, entry.Name()))
		items = append(items, fileItem{
			Name:        entry.Name(),
			Path:        itemPath,
			Size:        info.Size(),
			IsDirectory: entry.IsDir(),
			UpdatedAt:   info.ModTime(),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDirectory != items[j].IsDirectory {
			return items[i].IsDirectory
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})

	logger.Debug("list files", "request_id", requestID, "user_id", user.ID, "path", relativePath, "count", len(items))

	c.JSON(http.StatusOK, gin.H{
		"path":  relativePath,
		"items": items,
	})
}

func CreateFolder(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	var req createFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || strings.Contains(req.Name, "/") || strings.Contains(req.Name, "\\") {
		c.JSON(http.StatusBadRequest, gin.H{"message": "文件夹名称不合法"})
		return
	}

	_, parentPath, err := resolveUserPath(user.ID, req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}

	targetPath := filepath.Join(parentPath, req.Name)
	fullPath := filepath.ToSlash(filepath.Join(req.Path, req.Name))
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建文件夹失败"})
		return
	}

	logger.Info("create folder", "request_id", requestID, "user_id", user.ID, "path", fullPath)

	c.JSON(http.StatusOK, gin.H{
		"message": "创建成功",
	})
}

func UploadFile(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	relativePath, targetDir, err := resolveUserPath(user.ID, c.PostForm("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建目标目录失败"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请选择要上传的文件"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "打开上传文件失败"})
		return
	}
	defer src.Close()

	originalName := filepath.Base(fileHeader.Filename)
	safeName := originalName
	targetPath := filepath.Join(targetDir, safeName)
	renamed := false
	if _, err := os.Stat(targetPath); err == nil {
		ext := filepath.Ext(safeName)
		baseName := strings.TrimSuffix(safeName, ext)
		targetPath = filepath.Join(targetDir, baseName+"-"+uuid.NewString()+ext)
		safeName = filepath.Base(targetPath)
		renamed = true
	}

	dst, err := os.Create(targetPath)
	if err != nil {
		logger.Error("upload file failed", "request_id", requestID, "user_id", user.ID, "filename", originalName, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建目标文件失败"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		logger.Error("upload file failed", "request_id", requestID, "user_id", user.ID, "filename", originalName, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存文件失败"})
		return
	}

	fullFilePath := filepath.ToSlash(filepath.Join(relativePath, safeName))
	if renamed {
		logger.Warn("upload file renamed due to conflict", "request_id", requestID, "user_id", user.ID, "original_name", originalName, "new_name", safeName, "path", fullFilePath)
	}
	logger.Info("upload file", "request_id", requestID, "user_id", user.ID, "filename", safeName, "path", fullFilePath, "size", written)

	c.JSON(http.StatusOK, gin.H{
		"message": "上传成功",
		"file": gin.H{
			"name": safeName,
			"size": written,
		},
	})
}

func DownloadFile(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	relativePath, absolutePath, err := resolveUserPath(user.ID, c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}

	info, err := os.Stat(absolutePath)
	if err != nil || info.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"message": "文件不存在"})
		return
	}

	logger.Info("download file", "request_id", requestID, "user_id", user.ID, "filename", filepath.Base(relativePath), "path", relativePath)

	c.Header("Content-Type", utils.GetFileContentType(info.Name()))
	c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(relativePath)+"\"")
	c.File(absolutePath)
}

func DeleteFile(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	relativePath, absolutePath, err := resolveUserPath(user.ID, c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}

	if absolutePath == userStorageRoot(user.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能删除根目录"})
		return
	}

	logger.Warn("delete file/folder", "request_id", requestID, "user_id", user.ID, "path", relativePath)

	if err := os.RemoveAll(absolutePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func RenameFile(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	var req renameFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "请求参数格式错误"})
		return
	}

	fromRelative, fromPath, err := resolveUserPath(user.ID, req.From)
	if err != nil || fromRelative == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "原路径不合法"})
		return
	}
	toRelative, toPath, err := resolveUserPath(user.ID, req.To)
	if err != nil || toRelative == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "目标路径不合法"})
		return
	}
	if fromPath == userStorageRoot(user.ID) || toPath == userStorageRoot(user.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能重命名根目录"})
		return
	}
	if fromPath == toPath {
		c.JSON(http.StatusOK, gin.H{"message": "重命名成功"})
		return
	}
	if _, err := os.Stat(fromPath); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "原文件不存在"})
		return
	}
	if _, err := os.Stat(toPath); err == nil {
		c.JSON(http.StatusConflict, gin.H{"message": "目标名称已存在"})
		return
	}
	if err := os.MkdirAll(filepath.Dir(toPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建目标目录失败"})
		return
	}
	if err := os.Rename(fromPath, toPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "重命名失败"})
		return
	}

	logger.Info("rename file", "request_id", requestID, "user_id", user.ID, "old_path", fromRelative, "new_path", toRelative)

	c.JSON(http.StatusOK, gin.H{
		"message": "重命名成功",
		"from":    fromRelative,
		"to":      toRelative,
	})
}

func ClientCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"implemented": []string{
			"token_auth",
			"file_list",
			"create_folder",
			"upload_file",
			"download_file",
			"delete_file",
		},
		"planned": []string{
			"client_login_and_refresh_token",
			"directory_snapshot_compare",
			"incremental_sync_by_hash",
			"conflict_detection_and_resolution",
			"upload_resume",
			"download_resume",
			"device_binding",
			"sync_task_history",
		},
	})
}

func resolveUserPath(userID uint, relative string) (string, string, error) {
	root := userStorageRoot(userID)
	cleanRelative := utils.CleanRelativePath(relative)
	absolutePath := filepath.Clean(filepath.Join(root, cleanRelative))
	relativeRoot := filepath.Clean(root)

	if absolutePath != relativeRoot && !strings.HasPrefix(absolutePath, relativeRoot+string(os.PathSeparator)) {
		return "", "", os.ErrPermission
	}

	if err := os.MkdirAll(root, 0755); err != nil {
		return "", "", err
	}

	return cleanRelative, absolutePath, nil
}

func userStorageRoot(userID uint) string {
	return filepath.Join(config.AppConfig.Storage.RootDir, "user-"+utils.UintToString(userID))
}

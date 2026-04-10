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
	"zcopy-server-backend/middleware"
	"zcopy-server-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type createFolderRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

type fileItem struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	IsDirectory bool      `json:"isDirectory"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func ListFiles(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{
		"path":  relativePath,
		"items": items,
	})
}

func CreateFolder(c *gin.Context) {
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
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建文件夹失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "创建成功",
	})
}

func UploadFile(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	_, targetDir, err := resolveUserPath(user.ID, c.PostForm("path"))
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

	safeName := filepath.Base(fileHeader.Filename)
	targetPath := filepath.Join(targetDir, safeName)
	if _, err := os.Stat(targetPath); err == nil {
		ext := filepath.Ext(safeName)
		baseName := strings.TrimSuffix(safeName, ext)
		targetPath = filepath.Join(targetDir, baseName+"-"+uuid.NewString()+ext)
		safeName = filepath.Base(targetPath)
	}

	dst, err := os.Create(targetPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "创建目标文件失败"})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "保存文件失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "上传成功",
		"file": gin.H{
			"name": safeName,
			"size": written,
		},
	})
}

func DownloadFile(c *gin.Context) {
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

	c.Header("Content-Type", utils.GetFileContentType(info.Name()))
	c.Header("Content-Disposition", "attachment; filename=\""+filepath.Base(relativePath)+"\"")
	c.File(absolutePath)
}

func DeleteFile(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "未登录"})
		return
	}

	_, absolutePath, err := resolveUserPath(user.ID, c.Query("path"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "路径不合法"})
		return
	}

	if absolutePath == userStorageRoot(user.ID) {
		c.JSON(http.StatusBadRequest, gin.H{"message": "不能删除根目录"})
		return
	}

	if err := os.RemoveAll(absolutePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
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

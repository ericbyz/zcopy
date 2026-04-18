package fileprovider

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	stdsync "sync"
	"time"

	"golang.org/x/net/webdav"
	"zcopy-client-backend/models"
)

type remoteWebDAVFS struct {
	service *Service
	task    models.BackupTask
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
	service     *Service
	task        models.BackupTask
	relPath     string
	cleanupPath string
	closeOnce   stdsync.Once
	closeErr    error
}

func (s *Service) serveWebDAV(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/webdav/") {
		http.NotFound(w, r)
		return
	}
	user, password, ok := r.BasicAuth()
	if !ok || user != s.webdavUsername || password != s.webdavPassword {
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
	task, ok := s.store.Get(taskID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	handler := &webdav.Handler{
		Prefix:     "/webdav/" + taskID,
		FileSystem: &remoteWebDAVFS{service: s, task: task},
		LockSystem: webdav.NewMemLS(),
	}
	handler.ServeHTTP(w, r)
}

func (fs *remoteWebDAVFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	token := fs.service.tokens.GetToken()
	if token == "" {
		return os.ErrPermission
	}
	if err := fs.service.remote.EnsureRemotePath(joinRemotePath(fs.task.RemotePath, name), token); err != nil {
		return err
	}
	return os.MkdirAll(taskLocalPath(fs.task, name), 0755)
}

func (fs *remoteWebDAVFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	cleanRel := normalizeWebDAVPath(name)
	token := fs.service.tokens.GetToken()
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
					entryInfo, infoErr := entry.Info()
					if infoErr == nil {
						dirEntries = append(dirEntries, entryInfo)
					}
				}
				return &remoteWebDAVDirFile{info: info, entries: dirEntries}, nil
			}
			file, openErr := os.Open(taskLocalPath(fs.task, cleanRel))
			if openErr == nil {
				return &remoteWebDAVReadFile{File: file}, nil
			}
		}
		slog.Info("缓存未命中，从远程下载", "file_path", name)
		info, err := fs.service.remoteInfoForPath(fs.task, cleanRel, token)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			entries, err := fs.service.remoteDirEntries(fs.task, cleanRel, token)
			if err != nil {
				return nil, err
			}
			return &remoteWebDAVDirFile{info: info, entries: entries}, nil
		}
		localPath := fs.service.webdavTempPath(fs.task.ID, cleanRel)
		if err := fs.service.remote.DownloadRemoteFile(joinRemotePath(fs.task.RemotePath, cleanRel), localPath, token); err != nil {
			return nil, err
		}
		file, err := os.Open(localPath)
		if err != nil {
			return nil, err
		}
		return &remoteWebDAVReadFile{File: file, cleanupPath: localPath}, nil
	}

	slog.Info("文件写入操作", "file_path", name)
	localPath := fs.service.webdavTempPath(fs.task.ID, cleanRel)
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
			err := fs.service.remote.DownloadRemoteFile(joinRemotePath(fs.task.RemotePath, cleanRel), localPath, token)
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
		service:     fs.service,
		task:        fs.task,
		relPath:     cleanRel,
		cleanupPath: localPath,
	}, nil
}

func (fs *remoteWebDAVFS) RemoveAll(ctx context.Context, name string) error {
	token := fs.service.tokens.GetToken()
	if token == "" {
		return os.ErrPermission
	}
	if err := fs.service.deleteRemotePath(joinRemotePath(fs.task.RemotePath, name), token); err != nil {
		return err
	}
	_ = os.RemoveAll(taskLocalPath(fs.task, name))
	return nil
}

func (fs *remoteWebDAVFS) Rename(ctx context.Context, oldName string, newName string) error {
	token := fs.service.tokens.GetToken()
	if token == "" {
		return os.ErrPermission
	}
	if err := fs.service.renameRemotePath(joinRemotePath(fs.task.RemotePath, oldName), joinRemotePath(fs.task.RemotePath, newName), token); err != nil {
		return err
	}
	return renameLocalMirrorPath(fs.task, oldName, newName)
}

func (fs *remoteWebDAVFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	token := fs.service.tokens.GetToken()
	if token == "" {
		return nil, os.ErrPermission
	}
	if info, err := localInfoForPath(fs.task, name); err == nil {
		return info, nil
	}
	return fs.service.remoteInfoForPath(fs.task, name, token)
}

func (info remoteWebDAVInfo) Name() string       { return info.name }
func (info remoteWebDAVInfo) Size() int64        { return info.size }
func (info remoteWebDAVInfo) Mode() os.FileMode  { return info.mode }
func (info remoteWebDAVInfo) ModTime() time.Time { return info.modTime }
func (info remoteWebDAVInfo) IsDir() bool        { return info.mode.IsDir() }
func (info remoteWebDAVInfo) Sys() any           { return nil }

func (f *remoteWebDAVDirFile) Close() error                                 { return nil }
func (f *remoteWebDAVDirFile) Read(p []byte) (int, error)                   { return 0, io.EOF }
func (f *remoteWebDAVDirFile) Seek(offset int64, whence int) (int64, error) { return 0, nil }

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

func (f *remoteWebDAVDirFile) Stat() (os.FileInfo, error)  { return f.info, nil }
func (f *remoteWebDAVDirFile) Write(p []byte) (int, error) { return 0, errors.New("not writable") }

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
			token := f.service.tokens.GetToken()
			if token == "" {
				f.closeErr = os.ErrPermission
			} else {
				remotePath := joinRemotePath(f.task.RemotePath, f.relPath)
				remoteDir := path.Dir(remotePath)
				if remoteDir == "." {
					remoteDir = ""
				}
				if err := f.service.remote.EnsureRemotePath(remoteDir, token); err != nil {
					f.closeErr = err
				} else {
					_ = f.service.deleteRemotePath(remotePath, token)
					filename := path.Base(remotePath)
					f.closeErr = f.service.uploadFileReader(filename, remoteDir, f.File, token)
					if f.closeErr != nil {
						slog.Error("文件上传失败", "filename", filename, "error", f.closeErr)
					} else {
						slog.Info("文件上传完成", "filename", filename)
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

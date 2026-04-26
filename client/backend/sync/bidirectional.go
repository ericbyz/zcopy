package sync

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"zcopy-client-backend/models"
	"zcopy-client-backend/utils"
)

type remoteListResponse struct {
	Path  string `json:"path"`
	Items []struct {
		Name        string    `json:"name"`
		Path        string    `json:"path"`
		Size        int64     `json:"size"`
		IsDirectory bool      `json:"isDirectory"`
		UpdatedAt   time.Time `json:"updatedAt"`
	} `json:"items"`
}

func (e *Engine) syncBidirectionalTask(task models.BackupTask, token string) error {
	mode := syncMode(task)
	e.logs.Push("info", task, "", "双向同步开始：task_id="+task.ID+", task_name="+task.Name)
	report := e.startSync(&task, mode, "准备双向同步")

	localFiles, err := utils.CollectLocalFiles(task.LocalPath)
	if err != nil {
		return e.failSync(&task, report, "扫描本地目录失败", err, 0)
	}
	localMap := make(map[string]models.LocalFileItem, len(localFiles))
	for _, item := range localFiles {
		localMap[item.RelPath] = item
	}

	remoteMap, err := e.collectRemoteFiles(task.RemotePath, token)
	if err != nil {
		return e.failSync(&task, report, "扫描文件服务器目录失败", err, 0)
	}
	snapshot := LoadSnapshot(e.snapshotDir, task.ID, e.logs, task)

	pathSet := make(map[string]struct{})
	for rel := range snapshot.Files {
		pathSet[rel] = struct{}{}
	}
	for rel := range localMap {
		pathSet[rel] = struct{}{}
	}
	for rel := range remoteMap {
		pathSet[rel] = struct{}{}
	}

	paths := make([]string, 0, len(pathSet))
	for rel := range pathSet {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	report.TotalFiles = len(paths)

	for _, rel := range paths {
		action := chooseSyncAction(task, rel, snapshot.Files[rel], localMap, remoteMap)
		switch action {
		case "upload":
			if err := e.uploadSingleFile(task, rel, localMap[rel], token); err != nil {
				report.FailedFiles++
				report.FailedFilePaths = append(report.FailedFilePaths, rel)
				continue
			}
			report.UploadedFiles++
			report.TransferredBytes += localMap[rel].Size
		case "download":
			if err := e.downloadSingleFile(task, rel, remoteMap[rel], token); err != nil {
				report.FailedFiles++
				report.FailedFilePaths = append(report.FailedFilePaths, rel)
				continue
			}
			report.UploadedFiles++
			report.TransferredBytes += remoteMap[rel].Size
		case "delete_local":
			if err := deleteLocalPath(filepath.Join(task.LocalPath, filepath.FromSlash(rel))); err != nil {
				report.FailedFiles++
				report.FailedFilePaths = append(report.FailedFilePaths, rel)
				continue
			}
		case "delete_remote":
			if err := e.deleteRemotePath(joinRemotePath(task.RemotePath, rel), token); err != nil {
				report.FailedFiles++
				report.FailedFilePaths = append(report.FailedFilePaths, rel)
				continue
			}
		}
	}

	localFiles, err = utils.CollectLocalFiles(task.LocalPath)
	if err != nil {
		return e.failSync(&task, report, "刷新本地快照失败", err, report.UploadedFiles)
	}
	remoteMap, err = e.collectRemoteFiles(task.RemotePath, token)
	if err != nil {
		return e.failSync(&task, report, "刷新远端快照失败", err, report.UploadedFiles)
	}
	nextSnapshot := models.TaskSnapshot{Files: map[string]models.FileFingerprint{}}
	for _, item := range localFiles {
		nextSnapshot.Files[item.RelPath] = models.FileFingerprint{Size: item.Size, ModUnix: item.ModUnix}
	}
	for rel, fp := range remoteMap {
		if _, exists := nextSnapshot.Files[rel]; exists {
			continue
		}
		nextSnapshot.Files[rel] = fp
	}
	if err := SaveSnapshot(e.snapshotDir, task.ID, nextSnapshot, e.logs, task); err != nil {
		e.logs.Push("warn", task, "", "保存双向同步快照失败: "+err.Error())
	}
	if report.FailedFiles > 0 {
		return e.failSync(&task, report, "同步模式存在失败文件", errors.New("部分文件同步失败"), report.UploadedFiles)
	}
	return e.finishSuccessfulSync(&task, mode, report)
}

func chooseSyncAction(task models.BackupTask, rel string, snapshot models.FileFingerprint, localMap map[string]models.LocalFileItem, remoteMap map[string]models.FileFingerprint) string {
	localItem, localExists := localMap[rel]
	remoteItem, remoteExists := remoteMap[rel]
	snapshotExists := snapshot.ModUnix != 0 || snapshot.Size != 0

	switch {
	case localExists && remoteExists:
		if localItem.Size == remoteItem.Size && localItem.ModUnix == remoteItem.ModUnix {
			return ""
		}
		if snapshotExists {
			localChanged := localItem.Size != snapshot.Size || localItem.ModUnix != snapshot.ModUnix
			remoteChanged := remoteItem.Size != snapshot.Size || remoteItem.ModUnix != snapshot.ModUnix
			switch {
			case localChanged && !remoteChanged:
				return "upload"
			case !localChanged && remoteChanged:
				return "download"
			}
		}
		return resolveConflictAction(task, localItem.ModUnix, remoteItem.ModUnix)
	case localExists && !remoteExists:
		if !snapshotExists {
			return "upload"
		}
		localChanged := localItem.Size != snapshot.Size || localItem.ModUnix != snapshot.ModUnix
		if !localChanged {
			return "delete_local"
		}
		return resolveDeletionConflict(task, localItem.ModUnix, snapshot.ModUnix, true)
	case !localExists && remoteExists:
		if !snapshotExists {
			return "download"
		}
		remoteChanged := remoteItem.Size != snapshot.Size || remoteItem.ModUnix != snapshot.ModUnix
		if !remoteChanged {
			return "delete_remote"
		}
		return resolveDeletionConflict(task, remoteItem.ModUnix, snapshot.ModUnix, false)
	default:
		return ""
	}
}

func resolveConflictAction(task models.BackupTask, localModUnix, remoteModUnix int64) string {
	switch normalizedConflictMode(task.ConflictMode) {
	case models.ConflictLocalPriority:
		return "upload"
	case models.ConflictRemotePriority:
		return "download"
	default:
		if remoteModUnix > localModUnix {
			return "download"
		}
		return "upload"
	}
}

func resolveDeletionConflict(task models.BackupTask, changedModUnix, snapshotModUnix int64, localExists bool) string {
	switch normalizedConflictMode(task.ConflictMode) {
	case models.ConflictLocalPriority:
		if localExists {
			return "upload"
		}
		return "delete_remote"
	case models.ConflictRemotePriority:
		if localExists {
			return "delete_local"
		}
		return "download"
	default:
		if changedModUnix > snapshotModUnix {
			if localExists {
				return "upload"
			}
			return "download"
		}
		if localExists {
			return "delete_local"
		}
		return "delete_remote"
	}
}

func (e *Engine) collectRemoteFiles(basePath string, token string) (map[string]models.FileFingerprint, error) {
	result := make(map[string]models.FileFingerprint)
	var walk func(current string) error
	walk = func(current string) error {
		endpoint := "/files"
		remotePath := joinRemotePath(basePath, current)
		if clean := utils.NormalizeRemote(remotePath); clean != "" {
			endpoint += "?path=" + url.QueryEscape(clean)
		}
		data, status, err := e.remote.RawRequest(http.MethodGet, endpoint, nil, "", token)
		if err != nil {
			return err
		}
		if status < 200 || status >= 300 {
			return errors.New("读取文件服务器目录失败")
		}
		var payload remoteListResponse
		if err := json.Unmarshal(data, &payload); err != nil {
			return err
		}
		for _, item := range payload.Items {
			rel := strings.TrimPrefix(strings.Trim(item.Path, "/"), strings.Trim(utils.NormalizeRemote(basePath), "/"))
			rel = strings.TrimPrefix(strings.Trim(rel, "/"), "/")
			if item.IsDirectory {
				if err := walk(rel); err != nil {
					return err
				}
				continue
			}
			result[rel] = models.FileFingerprint{
				Size:    item.Size,
				ModUnix: item.UpdatedAt.Unix(),
			}
		}
		return nil
	}
	return result, walk("")
}

func (e *Engine) uploadSingleFile(task models.BackupTask, rel string, item models.LocalFileItem, token string) error {
	remoteDir := utils.NormalizeRemote(path.Dir(joinRemotePath(task.RemotePath, rel)))
	if remoteDir == "." {
		remoteDir = ""
	}
	if err := e.remote.EnsureRemotePath(remoteDir, token); err != nil {
		return err
	}
	return e.remote.UploadFile(item.AbsPath, remoteDir, token)
}

func (e *Engine) downloadSingleFile(task models.BackupTask, rel string, remote models.FileFingerprint, token string) error {
	localPath := filepath.Join(task.LocalPath, filepath.FromSlash(rel))
	if err := e.remote.DownloadRemoteFile(joinRemotePath(task.RemotePath, rel), localPath, token); err != nil {
		return err
	}
	modTime := time.Unix(remote.ModUnix, 0)
	return os.Chtimes(localPath, modTime, modTime)
}

func (e *Engine) deleteRemotePath(remotePath string, token string) error {
	endpoint := "/files?path=" + url.QueryEscape(utils.NormalizeRemote(remotePath))
	data, status, err := e.remote.RawRequest(http.MethodDelete, endpoint, nil, "", token)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		msg := utils.ParseJSONMessage(data)
		if msg == "" {
			msg = "删除文件服务器文件失败"
		}
		return errors.New(msg)
	}
	return nil
}

func joinRemotePath(base string, rel string) string {
	baseClean := utils.NormalizeRemote(base)
	relClean := strings.Trim(strings.TrimSpace(path.Clean("/"+rel)), "/")
	if relClean == "." {
		relClean = ""
	}
	switch {
	case baseClean == "":
		return relClean
	case relClean == "":
		return baseClean
	default:
		return utils.NormalizeRemote(baseClean + "/" + relClean)
	}
}

func deleteLocalPath(target string) error {
	if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func normalizedConflictMode(raw string) string {
	switch strings.TrimSpace(raw) {
	case models.ConflictLocalPriority:
		return models.ConflictLocalPriority
	case models.ConflictRemotePriority:
		return models.ConflictRemotePriority
	default:
		return models.ConflictLatestPriority
	}
}

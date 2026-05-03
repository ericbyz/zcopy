package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"zcopy-client-backend/models"
	"zcopy-client-backend/utils"
)

func (e *Engine) ReleaseLocalSpace(taskID string) (ReleaseResult, error) {
	task, err := e.loadTask(taskID)
	if err != nil {
		return ReleaseResult{}, err
	}
	snapshot := LoadSnapshot(e.snapshotDir, task.ID, e.logs, task)
	if len(snapshot.Files) == 0 {
		return ReleaseResult{}, ErrSnapshotMissing
	}
	files, err := utils.CollectLocalFiles(task.LocalPath)
	if err != nil {
		return ReleaseResult{}, err
	}
	currentMap := make(map[string]models.LocalFileItem, len(files))
	for _, item := range files {
		currentMap[item.RelPath] = item
	}

	result := ReleaseResult{Task: task}
	scannedFiles := len(snapshot.Files)
	for rel, fp := range snapshot.Files {
		item, exists := currentMap[rel]
		if !exists {
			continue
		}
		if item.Size != fp.Size || item.ModUnix != fp.ModUnix {
			result.SkippedFiles++
			continue
		}
		if err := removeLocalFile(item.AbsPath); err != nil {
			result.SkippedFiles++
			e.logs.Push("error", task, rel, "释放本地空间失败: "+err.Error())
			continue
		}
		result.ReleasedFiles++
		result.ReleasedBytes += item.Size
	}

	now := time.Now()
	task.Status = "idle"
	task.SyncReport = &models.SyncReport{
		State:      "idle",
		Mode:       "on_demand",
		Message:    "本地空间释放完成",
		StartedAt:  now,
		FinishedAt: &now,
	}
	task.UpdatedAt = now
	_ = e.store.Upsert(task)
	e.logs.Push("info", task, "", fmt.Sprintf("本地空间释放完成：scanned_files=%d, released_files=%d, skipped_files=%d", scannedFiles, result.ReleasedFiles, result.SkippedFiles))
	result.Task = task
	return result, nil
}

func (e *Engine) HydrateFromCloud(taskID string) (models.BackupTask, error) {
	task, err := e.loadTask(taskID)
	if err != nil {
		return models.BackupTask{}, err
	}
	remote, err := e.remoteForTask(task)
	if err != nil {
		return task, err
	}
	token, err := e.tokenForTask(task)
	if err != nil {
		return task, err
	}
	snapshot := LoadSnapshot(e.snapshotDir, task.ID, e.logs, task)
	if len(snapshot.Files) == 0 {
		return task, ErrSnapshotMissing
	}

	keys := make([]string, 0, len(snapshot.Files))
	for rel := range snapshot.Files {
		keys = append(keys, rel)
	}
	sort.Strings(keys)
	report := &models.SyncReport{
		State:      "syncing",
		Mode:       "on_demand_hydrate",
		Message:    "开始下载云端文件",
		StartedAt:  time.Now(),
		TotalFiles: len(keys),
	}
	task.Status = "syncing"
	task.SyncReport = report
	task.UpdatedAt = time.Now()
	_ = e.store.Upsert(task)

	currentFiles, _ := utils.CollectLocalFiles(task.LocalPath)
	currentMap := make(map[string]models.LocalFileItem, len(currentFiles))
	for _, item := range currentFiles {
		currentMap[item.RelPath] = item
	}

	var firstErr error
	failed := make([]string, 0)
	downloadedFiles := 0
	skippedFiles := 0
	totalBytes := int64(0)
	for _, rel := range keys {
		fp := snapshot.Files[rel]
		localPath := filepath.Join(task.LocalPath, filepath.FromSlash(rel))
		if item, exists := currentMap[rel]; exists && item.Size == fp.Size && item.ModUnix == fp.ModUnix {
			report.UploadedFiles++
			report.TransferredBytes += item.Size
			skippedFiles++
			continue
		}
		remoteFile := utils.NormalizeRemote(filepath.ToSlash(filepath.Join(task.RemotePath, rel)))
		if err := remote.DownloadRemoteFile(remoteFile, localPath, token); err != nil {
			report.FailedFiles++
			failed = append(failed, rel)
			if firstErr == nil {
				firstErr = err
			}
			e.logs.Push("error", task, rel, "下载云端文件失败: "+err.Error())
		} else {
			report.UploadedFiles++
			report.TransferredBytes += fp.Size
			downloadedFiles++
			totalBytes += fp.Size
		}
		report.SpeedBytesPerSec = utils.CalcSpeed(report.TransferredBytes, report.StartedAt)
		task.UpdatedAt = time.Now()
		_ = e.store.Upsert(task)
	}

	now := time.Now()
	if firstErr != nil {
		task.Status = "error"
		task.LastError = firstErr.Error()
		report.State = "failed"
		report.Message = "下载云端文件失败"
		report.FailedFilePaths = failed
		report.FinishedAt = &now
		task.SyncReport = report
		task.UpdatedAt = now
		_ = e.store.Upsert(task)
		return task, firstErr
	}

	task.Status = "idle"
	task.LastError = ""
	task.LastSyncAt = &now
	task.SyncReport = &models.SyncReport{
		State:      "idle",
		Mode:       "on_demand_hydrate",
		Message:    "云端文件下载完成",
		StartedAt:  now,
		FinishedAt: &now,
	}
	task.UpdatedAt = now
	_ = e.store.Upsert(task)
	e.logs.Push("info", task, "", fmt.Sprintf("云端文件下载完成：downloaded_files=%d, skipped_files=%d, total_bytes=%d", downloadedFiles, skippedFiles, totalBytes))
	return task, nil
}

var removeLocalFile = func(path string) error {
	return os.Remove(path)
}

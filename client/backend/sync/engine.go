package sync

import (
	"errors"
	"fmt"
	"path/filepath"
	stdsync "sync"
	"time"

	"zcopy-client-backend/auth"
	logpkg "zcopy-client-backend/log"
	"zcopy-client-backend/models"
	"zcopy-client-backend/proxy"
	"zcopy-client-backend/store"
	"zcopy-client-backend/utils"
)

var (
	ErrTaskNotFound    = errors.New("任务不存在")
	ErrUnauthorized    = errors.New("请先登录客户端")
	ErrSnapshotMissing = errors.New("同步快照不存在")
)

type SyncEngine interface {
	SyncTask(taskID string) error
	ReleaseLocalSpace(taskID string) (ReleaseResult, error)
	HydrateFromCloud(taskID string) (models.BackupTask, error)
}

type Engine struct {
	store       store.TaskRepository
	tokens      auth.TokenProvider
	multiProxy  *proxy.MultiServerProxy
	logs        logpkg.LogStore
	snapshotDir string

	mu      stdsync.Mutex
	syncing map[string]bool
}

type ReleaseResult struct {
	Task          models.BackupTask
	ReleasedFiles int
	ReleasedBytes int64
	SkippedFiles  int
}

func NewEngine(store store.TaskRepository, tokens auth.TokenProvider, multiProxy *proxy.MultiServerProxy, logs logpkg.LogStore, snapshotDir string) *Engine {
	return &Engine{
		store:       store,
		tokens:      tokens,
		multiProxy:  multiProxy,
		logs:        logs,
		snapshotDir: snapshotDir,
		syncing:     make(map[string]bool),
	}
}

func (e *Engine) remoteForTask(task models.BackupTask) (proxy.RemoteClient, error) {
	if task.ServerID == "" {
		return nil, errors.New("任务未分配服务器")
	}
	return e.multiProxy.GetClient(task.ServerID)
}

func (e *Engine) tokenForTask(task models.BackupTask) (string, error) {
	if task.ServerID == "" {
		return "", ErrUnauthorized
	}
	token, ok := e.tokens.GetToken(task.ServerID)
	if !ok || token == "" {
		return "", ErrUnauthorized
	}
	return token, nil
}

func (e *Engine) SyncTask(taskID string) error {
	task, err := e.loadTask(taskID)
	if err != nil {
		return err
	}
	remote, err := e.remoteForTask(task)
	if err != nil {
		return err
	}
	token, err := e.tokenForTask(task)
	if err != nil {
		return err
	}
	if !e.acquire(taskID, task) {
		return nil
	}
	defer e.release(taskID, task)

	if task.TaskMode == models.TaskModeSync {
		return e.syncBidirectionalTask(task, remote, token)
	}

	if task.CloudOnly {
		e.logs.Push("info", task, "", "全新模式任务跳过本地备份：task_id="+taskID)
		return nil
	}

	e.logs.Push("info", task, "", "同步开始：task_id="+taskID+", task_name="+task.Name+", local_path="+task.LocalPath+", remote_path="+task.RemotePath)

	mode := syncMode(task)
	report := e.startSync(&task, mode, "准备同步")
	files, pendingFiles, snapshot, err := e.collectPendingFiles(task, report)
	if err != nil {
		return e.failSync(&task, report, "扫描本地目录失败", err, 0)
	}
	if task.OnDemandSync && len(pendingFiles) == 0 {
		return e.finishNoopSync(&task, mode)
	}
	report.Message = "扫描完成，开始传输"
	_ = e.store.Upsert(task)

	if err := remote.EnsureRemotePath(task.RemotePath, token); err != nil {
		return e.failSync(&task, report, "创建远程目录失败", err, 0)
	}
	if err := e.uploadPendingFiles(&task, remote, token, files, pendingFiles, snapshot, report); err != nil {
		return err
	}
	return e.finishSuccessfulSync(&task, mode, report)
}

func (e *Engine) loadTask(taskID string) (models.BackupTask, error) {
	task, ok := e.store.Get(taskID)
	if !ok {
		return models.BackupTask{}, ErrTaskNotFound
	}
	return task, nil
}

func (e *Engine) acquire(taskID string, task models.BackupTask) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.syncing[taskID] {
		e.logs.Push("warn", task, "", "同步已在进行中，跳过：task_id="+taskID)
		return false
	}
	e.syncing[taskID] = true
	return true
}

func (e *Engine) release(taskID string, task models.BackupTask) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs.Push("debug", task, "", "同步锁释放：task_id="+taskID)
	delete(e.syncing, taskID)
}

func syncMode(task models.BackupTask) string {
	if task.TaskMode == models.TaskModeSync {
		if task.OnDemandSync {
			return "sync_on_demand"
		}
		return "sync_full"
	}
	if task.CloudOnly {
		return "cloud_only"
	}
	if task.OnDemandSync {
		return "on_demand"
	}
	return "full"
}

func (e *Engine) startSync(task *models.BackupTask, mode string, message string) *models.SyncReport {
	report := &models.SyncReport{
		State:     "syncing",
		Mode:      mode,
		Message:   message,
		StartedAt: time.Now(),
	}
	task.Status = "syncing"
	task.SyncReport = report
	task.UpdatedAt = time.Now()
	_ = e.store.Upsert(*task)
	return report
}

func (e *Engine) collectPendingFiles(task models.BackupTask, report *models.SyncReport) ([]models.LocalFileItem, []models.LocalFileItem, models.TaskSnapshot, error) {
	files, err := utils.CollectLocalFiles(task.LocalPath)
	if err != nil {
		return nil, nil, models.TaskSnapshot{}, err
	}
	snapshot := models.TaskSnapshot{Files: map[string]models.FileFingerprint{}}
	if task.OnDemandSync {
		snapshot = LoadSnapshot(e.snapshotDir, task.ID, e.logs, task)
	}

	pendingFiles := make([]models.LocalFileItem, 0, len(files))
	skippedCount := 0
	for _, item := range files {
		shouldUpload := true
		if task.OnDemandSync {
			if fp, exists := snapshot.Files[item.RelPath]; exists && fp.Size == item.Size && fp.ModUnix == item.ModUnix {
				shouldUpload = false
			}
		}
		if !shouldUpload {
			skippedCount++
			continue
		}
		pendingFiles = append(pendingFiles, item)
		report.TotalFiles++
		report.TotalBytes += item.Size
	}

	totalFiles := len(files)
	pendingCount := len(pendingFiles)
	e.logs.Push("info", task, "", fmt.Sprintf("文件扫描完成：total_files=%d, pending_count=%d, skipped_count=%d", totalFiles, pendingCount, skippedCount))

	return files, pendingFiles, snapshot, nil
}

func (e *Engine) uploadPendingFiles(task *models.BackupTask, remote proxy.RemoteClient, token string, files []models.LocalFileItem, pendingFiles []models.LocalFileItem, snapshot models.TaskSnapshot, report *models.SyncReport) error {
	var firstErr error
	failedFilePaths := make([]string, 0)
	totalCount := len(pendingFiles)
	for i, item := range pendingFiles {
		remoteDir := utils.NormalizeRemote(filepath.ToSlash(filepath.Join(task.RemotePath, filepath.Dir(item.RelPath))))
		if remoteDir == "." {
			remoteDir = ""
		}
		if err := remote.UploadFile(item.AbsPath, remoteDir, token); err != nil {
			report.FailedFiles++
			failedFilePaths = append(failedFilePaths, item.RelPath)
			e.logs.Push("error", *task, item.RelPath, err.Error())
			if firstErr == nil {
				firstErr = err
			}
		} else {
			report.UploadedFiles++
			report.TransferredBytes += item.Size
		}
		snapshot.Files[item.RelPath] = models.FileFingerprint{
			Size:    item.Size,
			ModUnix: item.ModUnix,
		}
		report.SpeedBytesPerSec = utils.CalcSpeed(report.TransferredBytes, report.StartedAt)
		report.Message = "同步进行中"
		task.UpdatedAt = time.Now()
		_ = e.store.Upsert(*task)

		// 每50个文件记录一次进度
		if (i+1)%50 == 0 {
			percentage := 0
			if totalCount > 0 {
				percentage = (i + 1) * 100 / totalCount
			}
			e.logs.Push("info", *task, "", fmt.Sprintf("同步进度：count=%d/%d, percentage=%d%%", i+1, totalCount, percentage))
		}
	}
	for _, item := range files {
		if _, exists := snapshot.Files[item.RelPath]; exists {
			continue
		}
		snapshot.Files[item.RelPath] = models.FileFingerprint{
			Size:    item.Size,
			ModUnix: item.ModUnix,
		}
	}
	if task.OnDemandSync {
		if err := SaveSnapshot(e.snapshotDir, task.ID, snapshot, e.logs, *task); err != nil {
			e.logs.Push("error", *task, "", "快照保存失败："+err.Error())
		}
	}
	if firstErr != nil {
		report.FailedFilePaths = failedFilePaths
		return e.failSync(task, report, "同步失败", firstErr, report.UploadedFiles)
	}
	return nil
}

func (e *Engine) finishNoopSync(task *models.BackupTask, mode string) error {
	now := time.Now()
	task.Status = "idle"
	task.LastError = ""
	task.LastSyncAt = &now
	task.SyncReport = &models.SyncReport{
		State:      "idle",
		Mode:       mode,
		Message:    "按需同步：无变更",
		StartedAt:  now,
		FinishedAt: &now,
	}
	task.UpdatedAt = now
	return e.store.Upsert(*task)
}

func (e *Engine) finishSuccessfulSync(task *models.BackupTask, mode string, report *models.SyncReport) error {
	now := time.Now()
	duration := now.Sub(report.StartedAt).Seconds()
	task.Status = "idle"
	task.LastError = ""
	task.LastSyncAt = &now
	task.SyncReport = &models.SyncReport{
		State:            "idle",
		Mode:             mode,
		Message:          "同步完成",
		StartedAt:        report.StartedAt,
		FinishedAt:       &now,
		UploadedFiles:    report.UploadedFiles,
		TransferredBytes: report.TransferredBytes,
		TotalFiles:       report.TotalFiles,
		TotalBytes:       report.TotalBytes,
	}
	task.UpdatedAt = now
	e.logs.Push("info", *task, "", fmt.Sprintf("任务同步完成：uploaded_count=%d, total_bytes=%d, duration_seconds=%.2f", report.UploadedFiles, report.TransferredBytes, duration))
	return e.store.Upsert(*task)
}

func (e *Engine) failSync(task *models.BackupTask, report *models.SyncReport, message string, cause error, uploadedCount int) error {
	now := time.Now()
	task.Status = "error"
	task.LastError = cause.Error()
	task.SyncReport = report
	task.UpdatedAt = now
	report.State = "failed"
	report.Message = message
	report.FinishedAt = &now
	_ = e.store.Upsert(*task)
	e.logs.Push("error", *task, "", fmt.Sprintf("同步失败：task_id=%s, error_message=%s, uploaded_count_before_failure=%d", task.ID, cause.Error(), uploadedCount))
	return cause
}

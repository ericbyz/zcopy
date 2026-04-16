package sync

import (
	"errors"
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
	tokens      auth.TokenManager
	remote      proxy.RemoteClient
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

func NewEngine(store store.TaskRepository, tokens auth.TokenManager, remote proxy.RemoteClient, logs logpkg.LogStore, snapshotDir string) *Engine {
	return &Engine{
		store:       store,
		tokens:      tokens,
		remote:      remote,
		logs:        logs,
		snapshotDir: snapshotDir,
		syncing:     make(map[string]bool),
	}
}

func (e *Engine) SyncTask(taskID string) error {
	task, err := e.loadTask(taskID)
	if err != nil {
		return err
	}
	token := e.tokens.GetToken()
	if token == "" {
		return ErrUnauthorized
	}
	if !e.acquire(taskID) {
		return nil
	}
	defer e.release(taskID)

	mode := syncMode(task)
	report := e.startSync(&task, mode, "准备同步")
	files, pendingFiles, snapshot, err := e.collectPendingFiles(task, report)
	if err != nil {
		return e.failSync(&task, report, "扫描本地目录失败", err)
	}
	if task.OnDemandSync && len(pendingFiles) == 0 {
		return e.finishNoopSync(&task, mode)
	}
	report.Message = "扫描完成，开始传输"
	_ = e.store.Upsert(task)

	if err := e.remote.EnsureRemotePath(task.RemotePath, token); err != nil {
		return e.failSync(&task, report, "创建远程目录失败", err)
	}
	if err := e.uploadPendingFiles(&task, token, files, pendingFiles, snapshot, report); err != nil {
		return err
	}
	return e.finishSuccessfulSync(&task, mode)
}

func (e *Engine) loadTask(taskID string) (models.BackupTask, error) {
	task, ok := e.store.Get(taskID)
	if !ok {
		return models.BackupTask{}, ErrTaskNotFound
	}
	return task, nil
}

func (e *Engine) acquire(taskID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.syncing[taskID] {
		return false
	}
	e.syncing[taskID] = true
	return true
}

func (e *Engine) release(taskID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.syncing, taskID)
}

func syncMode(task models.BackupTask) string {
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
		snapshot = LoadSnapshot(e.snapshotDir, task.ID)
	}

	pendingFiles := make([]models.LocalFileItem, 0, len(files))
	for _, item := range files {
		shouldUpload := true
		if task.OnDemandSync {
			if fp, exists := snapshot.Files[item.RelPath]; exists && fp.Size == item.Size && fp.ModUnix == item.ModUnix {
				shouldUpload = false
			}
		}
		if !shouldUpload {
			continue
		}
		pendingFiles = append(pendingFiles, item)
		report.TotalFiles++
		report.TotalBytes += item.Size
	}
	return files, pendingFiles, snapshot, nil
}

func (e *Engine) uploadPendingFiles(task *models.BackupTask, token string, files []models.LocalFileItem, pendingFiles []models.LocalFileItem, snapshot models.TaskSnapshot, report *models.SyncReport) error {
	var firstErr error
	failedFilePaths := make([]string, 0)
	for _, item := range pendingFiles {
		remoteDir := utils.NormalizeRemote(filepath.ToSlash(filepath.Join(task.RemotePath, filepath.Dir(item.RelPath))))
		if remoteDir == "." {
			remoteDir = ""
		}
		if err := e.remote.UploadFile(item.AbsPath, remoteDir, token); err != nil {
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
		_ = SaveSnapshot(e.snapshotDir, task.ID, snapshot)
	}
	if firstErr != nil {
		report.FailedFilePaths = failedFilePaths
		e.logs.Push("error", *task, "", "任务同步失败")
		return e.failSync(task, report, "同步失败", firstErr)
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

func (e *Engine) finishSuccessfulSync(task *models.BackupTask, mode string) error {
	now := time.Now()
	task.Status = "idle"
	task.LastError = ""
	task.LastSyncAt = &now
	task.SyncReport = &models.SyncReport{
		State:      "idle",
		Mode:       mode,
		Message:    "同步完成",
		StartedAt:  now,
		FinishedAt: &now,
	}
	task.UpdatedAt = now
	e.logs.Push("info", *task, "", "任务同步完成")
	return e.store.Upsert(*task)
}

func (e *Engine) failSync(task *models.BackupTask, report *models.SyncReport, message string, cause error) error {
	now := time.Now()
	task.Status = "error"
	task.LastError = cause.Error()
	task.SyncReport = report
	task.UpdatedAt = now
	report.State = "failed"
	report.Message = message
	report.FinishedAt = &now
	_ = e.store.Upsert(*task)
	return cause
}

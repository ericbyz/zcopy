package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"zcopy-client-backend/models"
	"zcopy-client-backend/store"
)

type WatchManager interface {
	StartWatcher(taskID string, task models.BackupTask) error
	StopWatcher(taskID string)
	RestoreAutoWatchers()
}

type FSNotifyWatchManager struct {
	store    store.TaskRepository
	watchers map[string]*models.WatchController
	onSync   func(taskID string) error
}

func NewFSNotifyWatchManager(store store.TaskRepository, onSync func(taskID string) error) *FSNotifyWatchManager {
	return &FSNotifyWatchManager{
		store:    store,
		watchers: make(map[string]*models.WatchController),
		onSync:   onSync,
	}
}

func (w *FSNotifyWatchManager) StartWatcher(taskID string, task models.BackupTask) error {
	w.StopWatcher(taskID)
	ctrl := &models.WatchController{
		StopCh: make(chan struct{}),
		DoneCh: make(chan struct{}),
	}
	w.watchers[taskID] = ctrl
	slog.Info("文件监听已启动", "task_id", taskID, "local_path", task.LocalPath)
	go w.runWatcher(task, ctrl)
	go func() {
		_ = w.onSync(taskID)
	}()
	return nil
}

func (w *FSNotifyWatchManager) StopWatcher(taskID string) {
	if ctrl, ok := w.watchers[taskID]; ok {
		close(ctrl.StopCh)
		delete(w.watchers, taskID)
		slog.Info("文件监听已停止", "task_id", taskID)
	}
}

func (w *FSNotifyWatchManager) RestoreAutoWatchers() {
	count := 0
	for _, task := range w.store.List() {
		if task.AutoBackup {
			if err := w.StartWatcher(task.ID, task); err != nil {
				slog.Error("恢复自动监听失败", "task_id", task.ID, "error", err)
			} else {
				count++
			}
		}
	}
	slog.Info("恢复自动监听", "count", count)
}

func (w *FSNotifyWatchManager) runWatcher(task models.BackupTask, ctrl *models.WatchController) {
	defer close(ctrl.DoneCh)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}
	defer watcher.Close()

	_ = filepath.Walk(task.LocalPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			_ = watcher.Add(path)
		}
		return nil
	})

	timer := time.NewTimer(24 * time.Hour)
	timer.Stop()
	pending := false
	for {
		select {
		case <-ctrl.StopCh:
			return
		case evt := <-watcher.Events:
			if evt.Op&(fsnotify.Create) != 0 {
				if info, statErr := os.Stat(evt.Name); statErr == nil && info.IsDir() {
					slog.Debug("添加新子目录监听", "task_id", task.ID, "path", evt.Name)
					_ = watcher.Add(evt.Name)
				}
			}
			if !pending {
				pending = true
				timer.Reset(2 * time.Second)
			}
		case <-timer.C:
			pending = false
			slog.Info("防抖触发同步", "task_id", task.ID)
			_ = w.onSync(task.ID)
		case err := <-watcher.Errors:
			slog.Warn("文件监听错误", "task_id", task.ID, "error", err)
		}
	}
}

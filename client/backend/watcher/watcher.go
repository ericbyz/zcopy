package watcher

import (
	"log"
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
	}
}

func (w *FSNotifyWatchManager) RestoreAutoWatchers() {
	for _, task := range w.store.List() {
		if task.AutoBackup {
			if err := w.StartWatcher(task.ID, task); err != nil {
				log.Printf("failed to restore watcher for %s: %v", task.ID, err)
			}
		}
	}
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
					_ = watcher.Add(evt.Name)
				}
			}
			if !pending {
				pending = true
				timer.Reset(2 * time.Second)
			}
		case <-timer.C:
			pending = false
			_ = w.onSync(task.ID)
		case <-watcher.Errors:
		}
	}
}

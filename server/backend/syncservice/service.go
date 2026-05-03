package syncservice

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"zcopy-server-backend/logger"
)

type SyncTask struct {
	UserID       uint      `json:"userId"`
	ClientTaskID string    `json:"clientTaskId"`
	TaskName     string    `json:"taskName"`
	RemotePath   string    `json:"remotePath"`
	ConflictMode string    `json:"conflictMode,omitempty"`
	OnDemandSync bool      `json:"onDemandSync,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type Event struct {
	TaskID      string    `json:"taskId"`
	TaskName    string    `json:"taskName"`
	RemotePath  string    `json:"remotePath"`
	Path        string    `json:"path"`
	Type        string    `json:"type"`
	IsDirectory bool      `json:"isDirectory"`
	OccurredAt  time.Time `json:"occurredAt"`
}

type watchController struct {
	stopCh chan struct{}
}

type taskStore struct {
	mu    sync.RWMutex
	path  string
	tasks []SyncTask
}

type Manager struct {
	storageRoot string
	store       *taskStore

	mu          sync.Mutex
	watchers    map[string]*watchController
	subscribers map[uint]map[chan Event]struct{}
}

var Service *Manager

func Init(storageRoot, dataPath string) error {
	store := &taskStore{path: dataPath, tasks: make([]SyncTask, 0)}
	if err := store.load(); err != nil {
		return err
	}
	Service = &Manager{
		storageRoot: storageRoot,
		store:       store,
		watchers:    make(map[string]*watchController),
		subscribers: make(map[uint]map[chan Event]struct{}),
	}
	Service.restoreWatchers()
	return nil
}

func (m *Manager) Upsert(task SyncTask) error {
	if m == nil {
		return errors.New("sync manager not initialized")
	}
	task.RemotePath = normalizeRemote(task.RemotePath)
	task.TaskName = strings.TrimSpace(task.TaskName)
	task.UpdatedAt = time.Now()
	if err := os.MkdirAll(m.absoluteTaskPath(task), 0755); err != nil {
		return err
	}
	if err := m.store.upsert(task); err != nil {
		return err
	}
	m.startWatcher(task)
	logger.Info("sync task registered", "user_id", task.UserID, "task_id", task.ClientTaskID, "task_name", task.TaskName, "remote_path", task.RemotePath)
	return nil
}

func (m *Manager) Remove(userID uint, clientTaskID string) error {
	if m == nil {
		return errors.New("sync manager not initialized")
	}
	task, err := m.store.remove(userID, strings.TrimSpace(clientTaskID))
	if err != nil {
		return err
	}
	m.stopWatcher(task.UserID, task.ClientTaskID)
	logger.Info("sync task removed", "user_id", task.UserID, "task_id", task.ClientTaskID, "remote_path", task.RemotePath)
	return nil
}

func (m *Manager) OccupiedBy(userID uint, remotePath string, excludeTaskID string) (SyncTask, bool) {
	if m == nil {
		return SyncTask{}, false
	}
	clean := normalizeRemote(remotePath)
	for _, task := range m.store.listByUser(userID) {
		if task.ClientTaskID == strings.TrimSpace(excludeTaskID) {
			continue
		}
		if normalizeRemote(task.RemotePath) == clean {
			return task, true
		}
	}
	return SyncTask{}, false
}

func (m *Manager) GetTask(userID uint, clientTaskID string) (SyncTask, bool) {
	if m == nil {
		return SyncTask{}, false
	}
	taskID := strings.TrimSpace(clientTaskID)
	for _, task := range m.store.listByUser(userID) {
		if task.ClientTaskID == taskID {
			return task, true
		}
	}
	return SyncTask{}, false
}

func (m *Manager) Subscribe(userID uint) (chan Event, func()) {
	ch := make(chan Event, 32)
	m.mu.Lock()
	if _, ok := m.subscribers[userID]; !ok {
		m.subscribers[userID] = make(map[chan Event]struct{})
	}
	m.subscribers[userID][ch] = struct{}{}
	m.mu.Unlock()
	return ch, func() {
		m.mu.Lock()
		if subs, ok := m.subscribers[userID]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(m.subscribers, userID)
			}
		}
		m.mu.Unlock()
		close(ch)
	}
}

func (m *Manager) restoreWatchers() {
	for _, task := range m.store.listAll() {
		m.startWatcher(task)
	}
}

func (m *Manager) startWatcher(task SyncTask) {
	key := watcherKey(task.UserID, task.ClientTaskID)
	m.stopWatcher(task.UserID, task.ClientTaskID)

	ctrl := &watchController{stopCh: make(chan struct{})}
	m.mu.Lock()
	m.watchers[key] = ctrl
	m.mu.Unlock()

	go m.runWatcher(task, ctrl)
}

func (m *Manager) stopWatcher(userID uint, clientTaskID string) {
	key := watcherKey(userID, clientTaskID)
	m.mu.Lock()
	ctrl, ok := m.watchers[key]
	if ok {
		delete(m.watchers, key)
	}
	m.mu.Unlock()
	if ok {
		close(ctrl.stopCh)
	}
}

func (m *Manager) runWatcher(task SyncTask, ctrl *watchController) {
	root := m.absoluteTaskPath(task)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logger.Error("sync watcher create failed", "task_id", task.ClientTaskID, "error", err.Error())
		return
	}
	defer watcher.Close()

	_ = filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || !info.IsDir() {
			return nil
		}
		_ = watcher.Add(path)
		return nil
	})

	logger.Info("sync watcher started", "user_id", task.UserID, "task_id", task.ClientTaskID, "task_name", task.TaskName, "remote_path", task.RemotePath)

	for {
		select {
		case <-ctrl.stopCh:
			logger.Info("sync watcher stopped", "user_id", task.UserID, "task_id", task.ClientTaskID)
			return
		case evt, ok := <-watcher.Events:
			if !ok {
				return
			}
			if evt.Op&fsnotify.Create != 0 {
				if info, statErr := os.Stat(evt.Name); statErr == nil && info.IsDir() {
					_ = watcher.Add(evt.Name)
				}
			}
			eventType, shouldPublish := mapEventType(evt.Op)
			if !shouldPublish {
				continue
			}
			rel, relErr := filepath.Rel(root, evt.Name)
			if relErr != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			if rel == "." {
				rel = ""
			}
			isDir := false
			if info, statErr := os.Stat(evt.Name); statErr == nil && info.IsDir() {
				isDir = true
			}
			event := Event{
				TaskID:      task.ClientTaskID,
				TaskName:    task.TaskName,
				RemotePath:  task.RemotePath,
				Path:        rel,
				Type:        eventType,
				IsDirectory: isDir,
				OccurredAt:  time.Now(),
			}
			logger.Info(
				"sync watcher event",
				"user_id", task.UserID,
				"task_id", task.ClientTaskID,
				"task_name", task.TaskName,
				"remote_path", task.RemotePath,
				"event_type", eventType,
				"path", rel,
				"is_directory", isDir,
			)
			delivered := m.publish(task.UserID, event)
			if delivered > 0 {
				logger.Info(
					"sync watcher event dispatched",
					"user_id", task.UserID,
					"task_id", task.ClientTaskID,
					"task_name", task.TaskName,
					"remote_path", task.RemotePath,
					"event_type", eventType,
					"path", rel,
					"subscriber_count", delivered,
				)
			} else {
				logger.Warn(
					"sync watcher event has no client subscriber",
					"user_id", task.UserID,
					"task_id", task.ClientTaskID,
					"task_name", task.TaskName,
					"remote_path", task.RemotePath,
					"event_type", eventType,
					"path", rel,
				)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logger.Warn("sync watcher error", "user_id", task.UserID, "task_id", task.ClientTaskID, "error", err.Error())
		}
	}
}

func (m *Manager) publish(userID uint, event Event) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	delivered := 0
	for ch := range m.subscribers[userID] {
		select {
		case ch <- event:
			delivered++
		default:
		}
	}
	return delivered
}

func (m *Manager) absoluteTaskPath(task SyncTask) string {
	return filepath.Join(m.storageRoot, fmt.Sprintf("user-%d", task.UserID), filepath.FromSlash(normalizeRemote(task.RemotePath)))
}

func watcherKey(userID uint, clientTaskID string) string {
	return fmt.Sprintf("%d:%s", userID, strings.TrimSpace(clientTaskID))
}

func mapEventType(op fsnotify.Op) (string, bool) {
	switch {
	case op&fsnotify.Create != 0:
		return "created", true
	case op&fsnotify.Write != 0:
		return "modified", true
	case op&fsnotify.Remove != 0:
		return "deleted", true
	case op&fsnotify.Rename != 0:
		return "deleted", true
	default:
		return "", false
	}
}

func normalizeRemote(raw string) string {
	clean := filepath.ToSlash(strings.TrimSpace(raw))
	clean = strings.TrimPrefix(clean, "/")
	clean = strings.TrimSuffix(clean, "/")
	if clean == "." {
		return ""
	}
	return clean
}

func (s *taskStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.flushLocked()
		}
		return err
	}
	if len(buf) == 0 {
		return s.flushLocked()
	}
	return json.Unmarshal(buf, &s.tasks)
}

func (s *taskStore) listAll() []SyncTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]SyncTask, len(s.tasks))
	copy(items, s.tasks)
	return items
}

func (s *taskStore) listByUser(userID uint) []SyncTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]SyncTask, 0)
	for _, task := range s.tasks {
		if task.UserID == userID {
			items = append(items, task)
		}
	}
	return items
}

func (s *taskStore) upsert(task SyncTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].UserID == task.UserID && s.tasks[i].ClientTaskID == task.ClientTaskID {
			s.tasks[i] = task
			return s.flushLocked()
		}
	}
	s.tasks = append(s.tasks, task)
	return s.flushLocked()
}

func (s *taskStore) remove(userID uint, clientTaskID string) (SyncTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].UserID == userID && s.tasks[i].ClientTaskID == clientTaskID {
			task := s.tasks[i]
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return task, s.flushLocked()
		}
	}
	return SyncTask{}, errors.New("sync task not found")
}

func (s *taskStore) flushLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

package store

import (
	"encoding/json"
	"errors"
	"os"
	"sync"

	"zcopy-client-backend/models"
)

type TaskRepository interface {
	List() []models.BackupTask
	Get(id string) (models.BackupTask, bool)
	Upsert(task models.BackupTask) error
	Remove(id string) error
}

type TaskStore struct {
	mu    sync.RWMutex
	path  string
	tasks []models.BackupTask
}

func New(path string) *TaskStore {
	return &TaskStore{
		path:  path,
		tasks: make([]models.BackupTask, 0),
	}
}

func (s *TaskStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	buf, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.tasks = []models.BackupTask{}
			return nil
		}
		return err
	}
	if len(buf) == 0 {
		s.tasks = []models.BackupTask{}
		return nil
	}
	return json.Unmarshal(buf, &s.tasks)
}

func (s *TaskStore) List() []models.BackupTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	items := make([]models.BackupTask, len(s.tasks))
	copy(items, s.tasks)
	return items
}

func (s *TaskStore) Get(id string) (models.BackupTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tasks {
		if t.ID == id {
			return t, true
		}
	}
	return models.BackupTask{}, false
}

func (s *TaskStore) Upsert(task models.BackupTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].ID == task.ID {
			s.tasks[i] = task
			return s.flushLocked()
		}
	}
	s.tasks = append(s.tasks, task)
	return s.flushLocked()
}

func (s *TaskStore) Remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := -1
	for i := range s.tasks {
		if s.tasks[i].ID == id {
			index = i
			break
		}
	}
	if index < 0 {
		return errors.New("not found")
	}
	s.tasks = append(s.tasks[:index], s.tasks[index+1:]...)
	return s.flushLocked()
}

func (s *TaskStore) flushLocked() error {
	data, err := json.MarshalIndent(s.tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}

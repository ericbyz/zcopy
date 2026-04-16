package log

import (
	"strings"
	"sync"
	"time"

	"zcopy-client-backend/models"
)

type LogStore interface {
	Push(level string, task models.BackupTask, filePath, message string)
	List(taskID, level string, limit int) []models.TransferLog
}

type RingBufferLogStore struct {
	mu   sync.Mutex
	logs []models.TransferLog
}

func NewRingBufferLogStore() *RingBufferLogStore {
	return &RingBufferLogStore{
		logs: make([]models.TransferLog, 0),
	}
}

func (s *RingBufferLogStore) Push(level string, task models.BackupTask, filePath, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry := models.TransferLog{
		ID:        "log-" + time.Now().Format("20060102150405.000000000"),
		TaskID:    task.ID,
		TaskName:  task.Name,
		Level:     level,
		FilePath:  filePath,
		Message:   message,
		CreatedAt: time.Now(),
	}
	s.logs = append(s.logs, entry)
	if len(s.logs) > 2000 {
		s.logs = s.logs[len(s.logs)-2000:]
	}
}

func (s *RingBufferLogStore) List(taskID, level string, limit int) []models.TransferLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]models.TransferLog, 0, limit)
	for i := len(s.logs) - 1; i >= 0 && len(result) < limit; i-- {
		item := s.logs[i]
		if taskID != "" && item.TaskID != taskID {
			continue
		}
		if level != "" && strings.ToLower(item.Level) != level {
			continue
		}
		result = append(result, item)
	}
	return result
}

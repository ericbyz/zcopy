package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"zcopy-client-backend/models"
)

type LogStore interface {
	Push(level string, task models.BackupTask, filePath, message string)
	List(taskID, level string, limit int) []models.TransferLog
	Query(q ListQuery) (*ListResult, error)
	Export(q ListQuery) (io.Reader, error)
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
	query := ListQuery{
		TaskID:   taskID,
		Level:    level,
		Page:     1,
		PageSize: limit,
	}
	result, _ := s.Query(query)
	return result.Items
}

func (s *RingBufferLogStore) Query(q ListQuery) (*ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allLogs []models.TransferLog

	for i := len(s.logs) - 1; i >= 0; i-- {
		item := s.logs[i]

		if q.TaskID != "" && item.TaskID != q.TaskID {
			continue
		}
		if q.Level != "" && strings.ToLower(item.Level) != q.Level {
			continue
		}
		if q.Keyword != "" && !strings.Contains(item.Message, q.Keyword) && !strings.Contains(item.FilePath, q.Keyword) {
			continue
		}
		if q.StartTime != nil && item.CreatedAt.Before(*q.StartTime) {
			continue
		}
		if q.EndTime != nil && item.CreatedAt.After(*q.EndTime) {
			continue
		}

		allLogs = append(allLogs, item)
	}

	total := len(allLogs)
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &ListResult{
		Items:    allLogs[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *RingBufferLogStore) Export(q ListQuery) (io.Reader, error) {
	result, err := s.Query(q)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()
	go func() {
		defer w.Close()
		encoder := json.NewEncoder(w)
		for _, log := range result.Items {
			if err := encoder.Encode(log); err != nil {
				return
			}
		}
	}()

	return r, nil
}

type ListQuery struct {
	TaskID    string
	Level     string
	Keyword   string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
}

type ListResult struct {
	Items    []models.TransferLog
	Total    int
	Page     int
	PageSize int
}

type FileLogStore struct {
	mu             sync.Mutex
	logDir         string
	currentFile    *os.File
	writeCh        chan models.TransferLog
	stopCh         chan struct{}
	closeOnce      sync.Once
	wg             sync.WaitGroup
	writeWg        sync.WaitGroup
	maxFileSize    int64
	retentionDays  int
	checkInterval  time.Duration
}

const (
	defaultMaxFileSize   = 100 * 1024 * 1024 // 100MB
	defaultRetentionDays = 30
	defaultCheckInterval = 1 * time.Hour
)

func NewFileLogStore(logDir string) *FileLogStore {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		panic(err)
	}

	store := &FileLogStore{
		logDir:        logDir,
		writeCh:       make(chan models.TransferLog, 1000),
		stopCh:        make(chan struct{}),
		maxFileSize:   defaultMaxFileSize,
		retentionDays: defaultRetentionDays,
		checkInterval: defaultCheckInterval,
	}

	if err := store.openCurrentFile(); err != nil {
		panic(err)
	}

	store.wg.Add(1)
	go store.writeLoop()

	return store
}

func (s *FileLogStore) openCurrentFile() error {
	today := time.Now().Format("2006-01-02")
	filePath := filepath.Join(s.logDir, today+".jsonl")

	if s.currentFile != nil {
		s.currentFile.Sync()
		s.currentFile.Close()
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	s.currentFile = f
	return nil
}

func (s *FileLogStore) writeLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case entry := <-s.writeCh:
			s.mu.Lock()
			s.writeEntry(entry)
			s.checkRotationAndRetention() // Check rotation after each write for testability
			s.mu.Unlock()
			s.writeWg.Done()
		case <-ticker.C:
			s.mu.Lock()
			s.checkRotationAndRetention()
			s.mu.Unlock()
		case <-s.stopCh:
			// Drain remaining entries
			s.mu.Lock()
			defer s.mu.Unlock()
		drainLoop:
			for {
				select {
				case entry := <-s.writeCh:
					s.writeEntry(entry)
					s.writeWg.Done()
				default:
					break drainLoop
				}
			}
			if s.currentFile != nil {
				s.currentFile.Sync()
				s.currentFile.Close()
			}
			return
		}
	}
}

func (s *FileLogStore) writeEntry(entry models.TransferLog) {
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	s.currentFile.Write(data)
}

func (s *FileLogStore) checkRotationAndRetention() {
	now := time.Now()

	// Check daily rotation
	today := now.Format("2006-01-02")
	currentFileName := filepath.Base(s.currentFile.Name())
	if currentFileName != today+".jsonl" {
		s.openCurrentFile()
	}

	// Check size rotation
	info, err := s.currentFile.Stat()
	if err == nil && info.Size() >= s.maxFileSize {
		s.rotateSize()
	}

	// Check retention
	s.cleanOldFiles()
}

func (s *FileLogStore) rotateSize() {
	baseName := filepath.Base(s.currentFile.Name())
	ext := filepath.Ext(baseName)
	nameWithoutExt := baseName[:len(baseName)-len(ext)]
	timestamp := time.Now().Format("20060102150405")
	newPath := filepath.Join(s.logDir, fmt.Sprintf("%s-%s%s", nameWithoutExt, timestamp, ext))

	s.currentFile.Sync()
	s.currentFile.Close()
	os.Rename(filepath.Join(s.logDir, baseName), newPath)
	s.openCurrentFile()
}

func (s *FileLogStore) cleanOldFiles() {
	cutoff := time.Now().AddDate(0, 0, -s.retentionDays)

	files, err := os.ReadDir(s.logDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		info, err := file.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(s.logDir, file.Name()))
		}
	}
}

func (s *FileLogStore) Push(level string, task models.BackupTask, filePath, message string) {
	entry := models.TransferLog{
		ID:        "log-" + time.Now().Format("20060102150405.000000000"),
		TaskID:    task.ID,
		TaskName:  task.Name,
		Level:     level,
		FilePath:  filePath,
		Message:   message,
		CreatedAt: time.Now(),
	}

	s.writeWg.Add(1)
	select {
	case s.writeCh <- entry:
	default:
		// If channel is full, drop the entry to avoid blocking
		s.writeWg.Done()
	}
}

// Flush waits for all pending writes to complete. Only for testing.
func (s *FileLogStore) Flush() {
	s.writeWg.Wait()
}

func (s *FileLogStore) List(taskID, level string, limit int) []models.TransferLog {
	query := ListQuery{
		TaskID:   taskID,
		Level:    level,
		Page:     1,
		PageSize: limit,
	}
	result, _ := s.Query(query)
	return result.Items
}

func (s *FileLogStore) Query(q ListQuery) (*ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allLogs []models.TransferLog

	files, err := os.ReadDir(s.logDir)
	if err != nil {
		return nil, err
	}

	// Sort files by modification time (newest first)
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := files[i].Info()
		infoJ, _ := files[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if !strings.HasSuffix(file.Name(), ".jsonl") {
			continue
		}

		logs, err := s.readLogsFromFile(filepath.Join(s.logDir, file.Name()), q)
		if err != nil {
			continue
		}
		allLogs = append(allLogs, logs...)
	}

	// Sort all logs by CreatedAt (newest first)
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].CreatedAt.After(allLogs[j].CreatedAt)
	})

	total := len(allLogs)
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &ListResult{
		Items:    allLogs[start:end],
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *FileLogStore) readLogsFromFile(path string, q ListQuery) ([]models.TransferLog, error) {
	var logs []models.TransferLog

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		var logEntry models.TransferLog
		if err := json.Unmarshal(line, &logEntry); err != nil {
			continue
		}

		// Apply filters
		if q.TaskID != "" && logEntry.TaskID != q.TaskID {
			continue
		}
		if q.Level != "" && strings.ToLower(logEntry.Level) != q.Level {
			continue
		}
		if q.Keyword != "" && !strings.Contains(logEntry.Message, q.Keyword) && !strings.Contains(logEntry.FilePath, q.Keyword) {
			continue
		}
		if q.StartTime != nil && logEntry.CreatedAt.Before(*q.StartTime) {
			continue
		}
		if q.EndTime != nil && logEntry.CreatedAt.After(*q.EndTime) {
			continue
		}

		logs = append(logs, logEntry)
	}

	// Reverse to get newest first in this file
	for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 {
		logs[i], logs[j] = logs[j], logs[i]
	}

	return logs, nil
}

func (s *FileLogStore) Export(q ListQuery) (io.Reader, error) {
	result, err := s.Query(q)
	if err != nil {
		return nil, err
	}

	r, w := io.Pipe()
	go func() {
		defer w.Close()
		encoder := json.NewEncoder(w)
		for _, log := range result.Items {
			if err := encoder.Encode(log); err != nil {
				return
			}
		}
	}()

	return r, nil
}

func (s *FileLogStore) Close() {
	s.closeOnce.Do(func() {
		close(s.stopCh)
		s.wg.Wait()
	})
}

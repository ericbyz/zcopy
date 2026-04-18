package logger

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

var (
	globalLogger *slog.Logger
	writer       *rotatingWriter
	initOnce     sync.Once
)

func resetInit() {
	initOnce = sync.Once{}
	globalLogger = nil
	if writer != nil {
		writer.Close()
		writer = nil
	}
}

const (
	defaultMaxAgeDays     = 30
	defaultMaxTotalSizeMB = 500
	defaultMaxFileSizeMB  = 100
	bufferChanCapacity    = 4096
)

type rotatingWriter struct {
	logDir          string
	maxFileSize     int64
	maxAgeDays      int
	maxTotalSizeMB  int
	currentDate     string
	currentFile     *os.File
	currentFileSize int64
	bufferChan      chan []byte
	stopChan        chan struct{}
	wg              sync.WaitGroup
	mu              sync.Mutex
}

func Init(levelStr, logDir string) {
	initOnce.Do(func() {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			globalLogger = slog.New(slog.NewJSONHandler(os.Stderr, nil))
			return
		}

		writer = &rotatingWriter{
			logDir:         logDir,
			maxFileSize:    defaultMaxFileSizeMB * 1024 * 1024,
			maxAgeDays:     defaultMaxAgeDays,
			maxTotalSizeMB: defaultMaxTotalSizeMB,
			currentDate:    time.Now().Format("2006-01-02"),
			bufferChan:     make(chan []byte, bufferChanCapacity),
			stopChan:       make(chan struct{}),
		}

		writer.wg.Add(1)
		go writer.processWrites()

		writer.cleanupOldFiles()

		level := parseLogLevel(levelStr)

		handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
			Level: level,
		})
		globalLogger = slog.New(handler)
	})
}

func parseLogLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func L() *slog.Logger {
	if globalLogger == nil {
		// Fallback to stderr logger if not initialized
		return slog.New(slog.NewJSONHandler(os.Stderr, nil))
	}
	return globalLogger
}

func Debug(msg string, attrs ...any) {
	L().Debug(msg, attrs...)
}

func Info(msg string, attrs ...any) {
	L().Info(msg, attrs...)
}

func Warn(msg string, attrs ...any) {
	L().Warn(msg, attrs...)
}

func Error(msg string, attrs ...any) {
	L().Error(msg, attrs...)
}

func Close() {
	if writer != nil {
		writer.Close()
	}
}

func (w *rotatingWriter) Write(p []byte) (n int, err error) {
	select {
	case w.bufferChan <- append([]byte(nil), p...):
		return len(p), nil
	case <-w.stopChan:
		// If stopped, write to stderr instead
		os.Stderr.Write(p)
		return len(p), nil
	}
}

func (w *rotatingWriter) processWrites() {
	defer w.wg.Done()

	for {
		select {
		case p := <-w.bufferChan:
			w.writeSync(p)
		case <-w.stopChan:
			// Drain remaining buffer
			for {
				select {
				case p := <-w.bufferChan:
					w.writeSync(p)
				default:
					return
				}
			}
		}
	}
}

func (w *rotatingWriter) writeSync(p []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Check date rotation
	today := time.Now().Format("2006-01-02")
	if today != w.currentDate {
		w.rotateDate(today)
	}

	// Ensure current file is open
	if w.currentFile == nil {
		if err := w.openCurrentFile(); err != nil {
			os.Stderr.Write(p)
			return
		}
	}

	// Check size rotation
	if w.currentFileSize+int64(len(p)) > w.maxFileSize {
		w.rotateSize()
	}

	// Write to file
	n, err := w.currentFile.Write(p)
	if err != nil {
		os.Stderr.Write(p)
		return
	}
	w.currentFileSize += int64(n)

	// Flush to disk
	w.currentFile.Sync()
}

func (w *rotatingWriter) openCurrentFile() error {
	filename := filepath.Join(w.logDir, w.currentDate+".jsonl")
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.currentFile = file

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		w.currentFileSize = 0
	} else {
		w.currentFileSize = info.Size()
	}
	return nil
}

func (w *rotatingWriter) rotateDate(newDate string) {
	if w.currentFile != nil {
		w.currentFile.Close()
		w.currentFile = nil
	}
	w.currentDate = newDate
	w.cleanupOldFiles()
}

func (w *rotatingWriter) rotateSize() {
	if w.currentFile != nil {
		w.currentFile.Close()
		w.currentFile = nil
	}

	baseName := filepath.Join(w.logDir, w.currentDate+".jsonl")
	suffix := 1
	for {
		rotatedName := baseName + "." + strconv.Itoa(suffix)
		if _, err := os.Stat(rotatedName); os.IsNotExist(err) {
			os.Rename(baseName, rotatedName)
			break
		}
		suffix++
	}
}

func (w *rotatingWriter) cleanupOldFiles() {
	// List all log files
	entries, err := os.ReadDir(w.logDir)
	if err != nil {
		return
	}

	type fileInfo struct {
		path    string
		modTime time.Time
		size    int64
	}
	var files []fileInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(w.logDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, fileInfo{
			path:    path,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	// Sort by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.Before(files[j].modTime)
	})

	// Delete files older than maxAgeDays
	cutoff := time.Now().AddDate(0, 0, -w.maxAgeDays)
	var remainingFiles []fileInfo
	for _, fi := range files {
		if fi.modTime.Before(cutoff) {
			os.Remove(fi.path)
		} else {
			remainingFiles = append(remainingFiles, fi)
		}
	}

	// Check total size
	totalSize := int64(0)
	for _, fi := range remainingFiles {
		totalSize += fi.size
	}

	maxTotalSize := int64(w.maxTotalSizeMB) * 1024 * 1024
	if totalSize > maxTotalSize {
		// Delete oldest files until we're under limit
		for _, fi := range remainingFiles {
			if totalSize <= maxTotalSize {
				break
			}
			os.Remove(fi.path)
			totalSize -= fi.size
		}
	}
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	select {
	case <-w.stopChan:
		// Already closed
		w.mu.Unlock()
		return nil
	default:
		close(w.stopChan)
	}
	w.mu.Unlock()

	// Wait for background goroutine to finish
	w.wg.Wait()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.currentFile != nil {
		w.currentFile.Close()
		w.currentFile = nil
	}
	return nil
}

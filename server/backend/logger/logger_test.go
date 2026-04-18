package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestJSONLFormat(t *testing.T) {
	resetInit()
	tempDir := t.TempDir()
	Init("debug", tempDir)
	defer Close()

	testMsg := "test message"
	Info(testMsg, "user_id", 123, "operation", "test")

	time.Sleep(100 * time.Millisecond)

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(tempDir, today+".jsonl")
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))
	if len(lines) < 1 {
		t.Fatal("No log lines found")
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(lines[0], &entry); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if entry["msg"] != testMsg {
		t.Errorf("Expected msg %q, got %q", testMsg, entry["msg"])
	}
	if entry["user_id"] != float64(123) {
		t.Errorf("Expected user_id 123, got %v", entry["user_id"])
	}
}

func TestRetentionCleanup(t *testing.T) {
	resetInit()
	tempDir := t.TempDir()

	oldFiles := []string{
		"2025-01-01.jsonl",
		"2025-01-01.jsonl.1",
		"2025-01-02.jsonl",
	}
	for _, f := range oldFiles {
		path := filepath.Join(tempDir, f)
		os.WriteFile(path, []byte("old"), 0644)
		oldTime := time.Now().Add(-40 * 24 * time.Hour)
		os.Chtimes(path, oldTime, oldTime)
	}

	recentFile := filepath.Join(tempDir, time.Now().Format("2006-01-02")+".jsonl")
	os.WriteFile(recentFile, []byte("recent"), 0644)

	Init("info", tempDir)
	defer Close()
	time.Sleep(100 * time.Millisecond)

	for _, f := range oldFiles {
		path := filepath.Join(tempDir, f)
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("Old file %q should have been deleted", f)
		}
	}

	if _, err := os.Stat(recentFile); os.IsNotExist(err) {
		t.Error("Recent file should not have been deleted")
	}
}

func TestConcurrentWrites(t *testing.T) {
	resetInit()
	tempDir := t.TempDir()
	Init("debug", tempDir)
	defer Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	msgsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < msgsPerGoroutine; j++ {
				Info("concurrent test", "goroutine", id, "msg_num", j)
			}
		}(i)
	}
	wg.Wait()

	Close()

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(tempDir, today+".jsonl")
	content, _ := os.ReadFile(logFile)
	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))

	expectedLines := numGoroutines * msgsPerGoroutine
	if len(lines) != expectedLines {
		t.Errorf("Expected %d log lines, got %d", expectedLines, len(lines))
	}
}

func TestAsyncFlush(t *testing.T) {
	resetInit()
	tempDir := t.TempDir()
	Init("debug", tempDir)

	for i := 0; i < 100; i++ {
		Info("flush test", "i", i)
	}

	Close()

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(tempDir, today+".jsonl")
	content, _ := os.ReadFile(logFile)
	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))

	if len(lines) != 100 {
		t.Errorf("Expected 100 log lines after flush, got %d", len(lines))
	}
}

func TestDiskFullGraceful(t *testing.T) {
	resetInit()
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "readonly")
	os.Mkdir(readOnlyDir, 0555)

	Init("info", readOnlyDir)
	Info("this should not panic")
	Close()
}

func TestLogLevelParsing(t *testing.T) {
	tests := []struct {
		levelStr  string
		wantLevel slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"DEBUG", slog.LevelDebug},
		{"INFO", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.levelStr, func(t *testing.T) {
			got := parseLogLevel(tt.levelStr)
			if got != tt.wantLevel {
				t.Errorf("parseLogLevel(%q) = %v, want %v", tt.levelStr, got, tt.wantLevel)
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	resetInit()
	tempDir := t.TempDir()
	Init("debug", tempDir)
	defer Close()

	Debug("debug msg", "key", "debug")
	Info("info msg", "key", "info")
	Warn("warn msg", "key", "warn")
	Error("error msg", "key", "error", "error", fmt.Errorf("test error"))

	time.Sleep(100 * time.Millisecond)

	today := time.Now().Format("2006-01-02")
	logFile := filepath.Join(tempDir, today+".jsonl")
	content, _ := os.ReadFile(logFile)
	lines := bytes.Split(bytes.TrimSpace(content), []byte("\n"))

	if len(lines) != 4 {
		t.Errorf("Expected 4 log lines from helpers, got %d", len(lines))
	}
}

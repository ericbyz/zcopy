package log

import (
	"fmt"
	"testing"
	"time"

	"zcopy-client-backend/models"
)

func TestFilePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store, push entries
	store1 := NewFileLogStore(tmpDir)
	defer store1.Close()

	task := models.BackupTask{ID: "task-1", Name: "Test Task"}
	store1.Push("info", task, "file1.txt", "Test message 1")
	store1.Push("error", task, "file2.txt", "Test message 2")
	store1.Flush()

	// Close store1 to ensure all writes are flushed
	store1.Close()

	// Create new store from same directory
	store2 := NewFileLogStore(tmpDir)
	defer store2.Close()

	// List entries
	logs := store2.List("task-1", "", 10)
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(logs))
	}

	if logs[0].Message != "Test message 2" {
		t.Errorf("Expected first log message to be 'Test message 2', got '%s'", logs[0].Message)
	}
	if logs[1].Message != "Test message 1" {
		t.Errorf("Expected second log message to be 'Test message 1', got '%s'", logs[1].Message)
	}
}

func TestListPagination(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileLogStore(tmpDir)
	defer store.Close()

	task := models.BackupTask{ID: "task-paginate", Name: "Paginate Task"}
	for i := 0; i < 50; i++ {
		store.Push("info", task, "", fmt.Sprintf("message-%d", i))
	}
	store.Flush()

	query := ListQuery{
		TaskID:   "task-paginate",
		Page:     1,
		PageSize: 10,
	}
	result, err := store.Query(query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.Total != 50 {
		t.Errorf("Expected total 50, got %d", result.Total)
	}
	if len(result.Items) != 10 {
		t.Errorf("Expected 10 items, got %d", len(result.Items))
	}
}

func TestKeywordSearch(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileLogStore(tmpDir)
	defer store.Close()

	task := models.BackupTask{ID: "task-keyword", Name: "Keyword Task"}
	store.Push("info", task, "", "This is a test message with apple")
	store.Push("info", task, "", "This is another test message with banana")
	store.Push("info", task, "", "This is a third test message with apple again")
	store.Flush()

	query := ListQuery{
		TaskID:  "task-keyword",
		Keyword: "apple",
	}
	result, err := store.Query(query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 results for keyword 'apple', got %d", result.Total)
	}
}

func TestTimeRangeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileLogStore(tmpDir)
	defer store.Close()

	task := models.BackupTask{ID: "task-time", Name: "Time Task"}

	// Push entries with known times
	store.Push("info", task, "", "message 1")
	time.Sleep(10 * time.Millisecond)
	t1 := time.Now()
	time.Sleep(10 * time.Millisecond)
	store.Push("info", task, "", "message 2")
	time.Sleep(10 * time.Millisecond)
	t2 := time.Now()
	time.Sleep(10 * time.Millisecond)
	store.Push("info", task, "", "message 3")

	query := ListQuery{
		TaskID:    "task-time",
		StartTime: &t1,
		EndTime:   &t2,
	}
	result, err := store.Query(query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	// Should have at least message 2
	if result.Total < 1 {
		t.Errorf("Expected at least 1 result in time range, got %d", result.Total)
	}
}

func TestConcurrentPush(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileLogStore(tmpDir)
	defer store.Close()

	task := models.BackupTask{ID: "task-concurrent", Name: "Concurrent Task"}
	const numGoroutines = 50
	const numPerGoroutine = 10

	done := make(chan struct{})
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer func() { done <- struct{}{} }()
			for j := 0; j < numPerGoroutine; j++ {
				store.Push("info", task, "", fmt.Sprintf("goroutine-%d-msg-%d", goroutineID, j))
			}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	store.Flush()

	query := ListQuery{
		TaskID: "task-concurrent",
	}
	result, err := store.Query(query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	expectedTotal := numGoroutines * numPerGoroutine
	if result.Total != expectedTotal {
		t.Errorf("Expected %d total logs, got %d", expectedTotal, result.Total)
	}
}

func TestRotationAndRetention(t *testing.T) {
	tmpDir := t.TempDir()
	store := NewFileLogStore(tmpDir)
	defer store.Close()

	// Override rotation settings for test
	store.maxFileSize = 1024       // 1KB
	store.retentionDays = 30        // Keep 30 days
	store.checkInterval = 10 * time.Millisecond

	task := models.BackupTask{ID: "task-rotation", Name: "Rotation Task"}
	for i := 0; i < 100; i++ {
		store.Push("info", task, "", fmt.Sprintf("long message to fill up the file %d - %s", i, string(make([]byte, 100))))
	}

	store.Flush()

	// Check that we can still query
	query := ListQuery{
		TaskID: "task-rotation",
	}
	result, err := store.Query(query)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if result.Total == 0 {
		t.Errorf("Expected some logs after rotation, got 0")
	}
}

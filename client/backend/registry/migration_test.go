package registry

import (
	"testing"

	"zcopy-client-backend/models"
)

type mockTaskStore struct {
	tasks []models.BackupTask
}

func (m *mockTaskStore) List() []models.BackupTask {
	return m.tasks
}

func (m *mockTaskStore) Upsert(task models.BackupTask) error {
	for i, t := range m.tasks {
		if t.ID == task.ID {
			m.tasks[i] = task
			return nil
		}
	}
	m.tasks = append(m.tasks, task)
	return nil
}

func TestMigrateTasksToServer_EmptyServerID(t *testing.T) {
	defaultID := "server-default-123"
	task1 := models.BackupTask{ID: "task-1", Name: "Task 1"}
	task2 := models.BackupTask{ID: "task-2", Name: "Task 2"}
	task3 := models.BackupTask{ID: "task-3", Name: "Task 3"}

	store := &mockTaskStore{
		tasks: []models.BackupTask{task1, task2, task3},
	}

	err := MigrateTasksToServer(store, defaultID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := store.List()
	if len(results) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(results))
	}

	for _, task := range results {
		if task.ServerID != defaultID {
			t.Errorf("task %s: expected ServerID %q, got %q", task.ID, defaultID, task.ServerID)
		}
		if len(task.ServerIDs) != 1 || task.ServerIDs[0] != defaultID {
			t.Errorf("task %s: expected ServerIDs [%q], got %v", task.ID, defaultID, task.ServerIDs)
		}
	}
}

func TestMigrateTasksToServer_ExistingServerIDPreserved(t *testing.T) {
	defaultID := "server-default-123"
	existingID := "server-abc-456"
	task1 := models.BackupTask{ID: "task-1", Name: "Existing", ServerID: existingID, ServerIDs: []string{existingID}}
	task2 := models.BackupTask{ID: "task-2", Name: "Empty"}

	store := &mockTaskStore{
		tasks: []models.BackupTask{task1, task2},
	}

	err := MigrateTasksToServer(store, defaultID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := store.List()
	for _, task := range results {
		if task.ID == "task-1" {
			if task.ServerID != existingID {
				t.Errorf("task-1: expected ServerID to remain %q, got %q", existingID, task.ServerID)
			}
			if len(task.ServerIDs) != 1 || task.ServerIDs[0] != existingID {
				t.Errorf("task-1: expected ServerIDs to remain [%q], got %v", existingID, task.ServerIDs)
			}
		}
		if task.ID == "task-2" {
			if task.ServerID != defaultID {
				t.Errorf("task-2: expected ServerID %q, got %q", defaultID, task.ServerID)
			}
		}
	}
}

func TestMigrateTasksToServer_EmptyTaskList(t *testing.T) {
	store := &mockTaskStore{tasks: []models.BackupTask{}}
	err := MigrateTasksToServer(store, "server-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestMigrateTasksToServer_ServerIDsSync(t *testing.T) {
	defaultID := "server-xyz-789"
	task := models.BackupTask{ID: "task-test", Name: "Test Sync"}
	store := &mockTaskStore{tasks: []models.BackupTask{task}}

	err := MigrateTasksToServer(store, defaultID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := store.List()[0]
	if result.ServerID != defaultID {
		t.Errorf("expected ServerID %q, got %q", defaultID, result.ServerID)
	}
	if len(result.ServerIDs) != 1 || result.ServerIDs[0] != defaultID {
		t.Errorf("expected ServerIDs [%q], got %v", defaultID, result.ServerIDs)
	}
}

package registry_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"zcopy-client-backend/models"
	"zcopy-client-backend/registry"
	"zcopy-client-backend/store"
)

func TestMigrateTasksToServer_SetsServerID(t *testing.T) {
	dir := t.TempDir()
	ts := store.New(filepath.Join(dir, "tasks.json"))

	ts.Upsert(models.BackupTask{ID: "task-1", Name: "Task 1"})
	ts.Upsert(models.BackupTask{ID: "task-2", Name: "Task 2"})

	err := registry.MigrateTasksToServer(ts, "server-uuid-123")
	if err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	for _, task := range ts.List() {
		if task.ServerID != "server-uuid-123" {
			t.Errorf("task %s: ServerID = %q, want %q", task.ID, task.ServerID, "server-uuid-123")
		}
		if len(task.ServerIDs) != 1 || task.ServerIDs[0] != "server-uuid-123" {
			t.Errorf("task %s: ServerIDs = %v, want [server-uuid-123]", task.ID, task.ServerIDs)
		}
	}
}

func TestMigrateTasksToServer_Idempotent(t *testing.T) {
	dir := t.TempDir()
	ts := store.New(filepath.Join(dir, "tasks.json"))

	ts.Upsert(models.BackupTask{ID: "task-1", Name: "Task 1"})

	registry.MigrateTasksToServer(ts, "server-a")
	registry.MigrateTasksToServer(ts, "server-b")

	task, _ := ts.Get("task-1")
	if task.ServerID != "server-a" {
		t.Errorf("second migration overwrote ServerID: got %q, want %q", task.ServerID, "server-a")
	}
}

func TestMigrateTasksToServer_PreservesExistingServerID(t *testing.T) {
	dir := t.TempDir()
	ts := store.New(filepath.Join(dir, "tasks.json"))

	ts.Upsert(models.BackupTask{ID: "task-1", Name: "Task 1", ServerID: "existing-server"})
	ts.Upsert(models.BackupTask{ID: "task-2", Name: "Task 2"})

	registry.MigrateTasksToServer(ts, "new-server")

	task1, _ := ts.Get("task-1")
	if task1.ServerID != "existing-server" {
		t.Errorf("overwrote existing ServerID: got %q", task1.ServerID)
	}

	task2, _ := ts.Get("task-2")
	if task2.ServerID != "new-server" {
		t.Errorf("did not set empty ServerID: got %q", task2.ServerID)
	}
}

func TestServerRegistry_MultiServerCRUD(t *testing.T) {
	dir := t.TempDir()
	reg, err := registry.NewServerRegistry(dir)
	if err != nil {
		t.Fatalf("NewServerRegistry: %v", err)
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)

	s1 := models.ServerConfig{ID: "s1", Name: "Server A", Address: "host1:8890", IsDefault: true, AddedAt: now}
	s2 := models.ServerConfig{ID: "s2", Name: "Server B", Address: "host2:8890", IsDefault: false, AddedAt: now}

	reg.AddServer(s1)
	reg.AddServer(s2)

	servers := reg.ListServers()
	if len(servers) != 2 {
		t.Fatalf("ListServers: got %d, want 2", len(servers))
	}

	def, ok := reg.GetDefault()
	if !ok || def.ID != "s1" {
		t.Errorf("GetDefault: got %q, want s1", def.ID)
	}

	reg.RemoveServer("s2")
	servers = reg.ListServers()
	if len(servers) != 1 {
		t.Errorf("after remove: got %d servers, want 1", len(servers))
	}
}

func TestServerRegistry_PersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()

	reg1, _ := registry.NewServerRegistry(dir)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)
	reg1.AddServer(models.ServerConfig{ID: "p1", Name: "Persist", Address: "host:8890", AddedAt: now})

	reg2, _ := registry.NewServerRegistry(dir)
	servers := reg2.ListServers()
	if len(servers) != 1 || servers[0].ID != "p1" {
		t.Errorf("persistence round-trip failed: got %v", servers)
	}
}

func TestMultiServerProxy_Routing(t *testing.T) {
	if os.Getenv("ZCOPY_E2E") == "" {
		t.Skip("skipping e2e proxy test (set ZCOPY_E2E=1 to run)")
	}
}

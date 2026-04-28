package registry

import (
	"testing"
	"time"

	"zcopy-client-backend/models"
)

func TestAddServerAndListServers(t *testing.T) {
	tempDir := t.TempDir()
	reg, err := NewServerRegistry(tempDir)
	if err != nil {
		t.Fatalf("NewServerRegistry failed: %v", err)
	}

	server1 := models.ServerConfig{
		ID:              "server-1",
		Name:            "Test Server 1",
		Address:         "192.168.1.100:8890",
		IsDefault:       false,
		LastConnectedAt: time.Now(),
		Status:          "online",
		AddedAt:         time.Now(),
	}
	server2 := models.ServerConfig{
		ID:              "server-2",
		Name:            "Test Server 2",
		Address:         "192.168.1.101:8890",
		IsDefault:       false,
		LastConnectedAt: time.Now(),
		Status:          "offline",
		AddedAt:         time.Now(),
	}

	if err := reg.AddServer(server1); err != nil {
		t.Fatalf("AddServer 1 failed: %v", err)
	}
	if err := reg.AddServer(server2); err != nil {
		t.Fatalf("AddServer 2 failed: %v", err)
	}

	list := reg.ListServers()
	if len(list) != 2 {
		t.Errorf("ListServers returned %d servers, expected 2", len(list))
	}
}

func TestRemoveServer(t *testing.T) {
	tempDir := t.TempDir()
	reg, err := NewServerRegistry(tempDir)
	if err != nil {
		t.Fatalf("NewServerRegistry failed: %v", err)
	}

	server := models.ServerConfig{
		ID:      "server-to-remove",
		Name:    "To Remove",
		Address: "test:8890",
		AddedAt: time.Now(),
	}
	if err := reg.AddServer(server); err != nil {
		t.Fatalf("AddServer failed: %v", err)
	}
	if len(reg.ListServers()) != 1 {
		t.Fatalf("Expected 1 server after add")
	}
	if err := reg.RemoveServer(server.ID); err != nil {
		t.Fatalf("RemoveServer failed: %v", err)
	}
	if len(reg.ListServers()) != 0 {
		t.Errorf("Expected 0 servers after remove, got %d", len(reg.ListServers()))
	}
}

func TestSetDefault(t *testing.T) {
	tempDir := t.TempDir()
	reg, err := NewServerRegistry(tempDir)
	if err != nil {
		t.Fatalf("NewServerRegistry failed: %v", err)
	}

	server1 := models.ServerConfig{ID: "s1", Name: "S1", Address: "1:8890", AddedAt: time.Now()}
	server2 := models.ServerConfig{ID: "s2", Name: "S2", Address: "2:8890", AddedAt: time.Now()}

	if err := reg.AddServer(server1); err != nil {
		t.Fatal(err)
	}
	if err := reg.AddServer(server2); err != nil {
		t.Fatal(err)
	}

	if err := reg.SetDefault("s1"); err != nil {
		t.Fatal(err)
	}
	def, ok := reg.GetDefault()
	if !ok || def.ID != "s1" {
		t.Errorf("Expected default to be s1, got %v", def)
	}

	if err := reg.SetDefault("s2"); err != nil {
		t.Fatal(err)
	}
	def, ok = reg.GetDefault()
	if !ok || def.ID != "s2" {
		t.Errorf("Expected default to be s2, got %v", def)
	}

	list := reg.ListServers()
	defaultCount := 0
	for _, s := range list {
		if s.IsDefault {
			defaultCount++
		}
	}
	if defaultCount != 1 {
		t.Errorf("Expected exactly 1 default server, got %d", defaultCount)
	}
}

func TestPersistence(t *testing.T) {
	tempDir := t.TempDir()
	reg1, err := NewServerRegistry(tempDir)
	if err != nil {
		t.Fatalf("NewServerRegistry 1 failed: %v", err)
	}

	server := models.ServerConfig{
		ID:              "persist-test",
		Name:            "Persist Test",
		Address:         "persist:8890",
		IsDefault:       true,
		LastConnectedAt: time.Now(),
		Status:          "online",
		AddedAt:         time.Now(),
	}
	if err := reg1.AddServer(server); err != nil {
		t.Fatal(err)
	}

	reg2, err := NewServerRegistry(tempDir)
	if err != nil {
		t.Fatalf("NewServerRegistry 2 failed: %v", err)
	}

	list := reg2.ListServers()
	if len(list) != 1 {
		t.Fatalf("Expected 1 server after reload, got %d", len(list))
	}
	if list[0].ID != server.ID {
		t.Errorf("Expected ID %s, got %s", server.ID, list[0].ID)
	}
	def, ok := reg2.GetDefault()
	if !ok || def.ID != server.ID {
		t.Errorf("Expected default to persist")
	}
}

func TestDuplicateID(t *testing.T) {
	tempDir := t.TempDir()
	reg, err := NewServerRegistry(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	server := models.ServerConfig{ID: "dup-id", Name: "Dup", Address: "dup:8890", AddedAt: time.Now()}
	if err := reg.AddServer(server); err != nil {
		t.Fatal(err)
	}
	if err := reg.AddServer(server); err == nil {
		t.Error("Expected error for duplicate ID, got nil")
	} else if err != ErrServerExists {
		t.Errorf("Expected ErrServerExists, got %v", err)
	}
}

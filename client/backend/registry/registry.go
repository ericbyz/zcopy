package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"zcopy-client-backend/models"
)

var (
	ErrServerNotFound = errors.New("server not found")
	ErrServerExists   = errors.New("server with this ID already exists")
)

type ServerRegistry struct {
	mu       sync.RWMutex
	servers  []models.ServerConfig
	filePath string
}

func NewServerRegistry(dataDir string) (*ServerRegistry, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	filePath := filepath.Join(dataDir, "servers.json")
	registry := &ServerRegistry{
		filePath: filePath,
		servers:  []models.ServerConfig{},
	}
	if err := registry.load(); err != nil {
		return nil, err
	}
	return registry, nil
}

func (r *ServerRegistry) load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := os.Stat(r.filePath); errors.Is(err, os.ErrNotExist) {
		return r.flushLocked()
	}

	content, err := os.ReadFile(r.filePath)
	if err != nil {
		return fmt.Errorf("read servers file: %w", err)
	}

	if len(content) == 0 {
		return r.flushLocked()
	}

	var data struct {
		Servers []models.ServerConfig `json:"servers"`
	}
	if err := json.Unmarshal(content, &data); err != nil {
		return fmt.Errorf("parse servers file: %w", err)
	}
	r.servers = data.Servers
	return nil
}

func (r *ServerRegistry) flush() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.flushLocked()
}

func (r *ServerRegistry) flushLocked() error {
	data := struct {
		Servers []models.ServerConfig `json:"servers"`
	}{
		Servers: r.servers,
	}
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.filePath, content, 0644)
}

func (r *ServerRegistry) AddServer(server models.ServerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range r.servers {
		if s.ID == server.ID {
			return ErrServerExists
		}
	}

	r.servers = append(r.servers, server)
	return r.flushLocked()
}

func (r *ServerRegistry) RemoveServer(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	newServers := []models.ServerConfig{}
	for _, s := range r.servers {
		if s.ID != id {
			newServers = append(newServers, s)
		} else {
			found = true
		}
	}

	if !found {
		return ErrServerNotFound
	}

	r.servers = newServers
	return r.flushLocked()
}

func (r *ServerRegistry) ListServers() []models.ServerConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]models.ServerConfig, len(r.servers))
	copy(result, r.servers)
	return result
}

func (r *ServerRegistry) GetServer(id string) (models.ServerConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, s := range r.servers {
		if s.ID == id {
			return s, true
		}
	}
	return models.ServerConfig{}, false
}

func (r *ServerRegistry) UpdateServer(server models.ServerConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	for i, s := range r.servers {
		if s.ID == server.ID {
			r.servers[i] = server
			found = true
			break
		}
	}

	if !found {
		return ErrServerNotFound
	}

	return r.flushLocked()
}

func (r *ServerRegistry) SetDefault(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	found := false
	for i, s := range r.servers {
		if s.ID == id {
			r.servers[i].IsDefault = true
			found = true
		} else {
			r.servers[i].IsDefault = false
		}
	}

	if !found {
		return ErrServerNotFound
	}

	return r.flushLocked()
}

func (r *ServerRegistry) GetDefault() (models.ServerConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, s := range r.servers {
		if s.IsDefault {
			return s, true
		}
	}
	return models.ServerConfig{}, false
}

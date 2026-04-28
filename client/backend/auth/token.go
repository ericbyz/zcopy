package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type TokenManager interface {
	GetToken() string
	SetToken(token string)
}

type TokenProvider interface {
	GetToken(serverID string) (string, bool)
	SetToken(serverID string, token string)
	RemoveToken(serverID string)
}

type InMemoryTokenManager struct {
	mu    sync.RWMutex
	token string
}

func NewInMemoryTokenManager() *InMemoryTokenManager {
	return &InMemoryTokenManager{}
}

func (m *InMemoryTokenManager) GetToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.token
}

func (m *InMemoryTokenManager) SetToken(token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.token = token
}

type MultiServerTokenManager struct {
	mu       sync.RWMutex
	tokens   map[string]string
	filePath string
}

func NewMultiServerTokenManager(dataDir string) *MultiServerTokenManager {
	var filePath string
	if dataDir != "" {
		filePath = filepath.Join(dataDir, "tokens.json")
	}
	return &MultiServerTokenManager{
		tokens:   make(map[string]string),
		filePath: filePath,
	}
}

func (m *MultiServerTokenManager) GetToken(serverID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	token, ok := m.tokens[serverID]
	return token, ok
}

func (m *MultiServerTokenManager) SetToken(serverID string, token string) {
	m.mu.Lock()
	m.tokens[serverID] = token
	m.mu.Unlock()
	_ = m.Save()
}

func (m *MultiServerTokenManager) RemoveToken(serverID string) {
	m.mu.Lock()
	delete(m.tokens, serverID)
	m.mu.Unlock()
	_ = m.Save()
}

func (m *MultiServerTokenManager) Save() error {
	if m.filePath == "" {
		return nil
	}
	m.mu.RLock()
	data, err := json.MarshalIndent(m.tokens, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(m.filePath, data, 0644)
}

func (m *MultiServerTokenManager) Load() error {
	if m.filePath == "" {
		return nil
	}
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return json.Unmarshal(data, &m.tokens)
}

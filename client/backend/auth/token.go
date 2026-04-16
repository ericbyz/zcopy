package auth

import (
	"sync"
)

type TokenManager interface {
	GetToken() string
	SetToken(token string)
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

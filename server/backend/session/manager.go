package session

import (
	"context"
	"sync"
	"time"

	"zcopy-server-backend/models"
)

type SessionManager struct {
	mu       sync.RWMutex
	sessions map[string]*models.ClientSession
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*models.ClientSession),
	}
}

func (sm *SessionManager) RegisterOrUpdate(session models.ClientSession) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	existing, ok := sm.sessions[session.SessionID]
	if ok {
		existing.LastHeartbeat = session.LastHeartbeat
		existing.ActiveTasks = session.ActiveTasks
		existing.Status = session.Status
		existing.ClientIP = session.ClientIP
		existing.ServerID = session.ServerID
	} else {
		sm.sessions[session.SessionID] = &session
	}
}

func (sm *SessionManager) ListOnline() []models.ClientSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var online []models.ClientSession
	for _, s := range sm.sessions {
		if s.Status == "online" {
			online = append(online, *s)
		}
	}
	return online
}

func (sm *SessionManager) Remove(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
}

func (sm *SessionManager) CleanupStale(timeout time.Duration) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for _, s := range sm.sessions {
		if s.Status == "online" && now.Sub(s.LastHeartbeat) > timeout {
			s.Status = "offline"
		}
	}
}

func (sm *SessionManager) StartCleanupLoop(ctx context.Context, interval time.Duration, timeout time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.CleanupStale(timeout)
		case <-ctx.Done():
			return
		}
	}
}

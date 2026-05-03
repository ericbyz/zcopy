package session

import (
	"context"
	"sync"
	"testing"
	"time"

	"zcopy-server-backend/models"
)

func TestRegisterOrUpdate(t *testing.T) {
	sm := NewSessionManager()
	now := time.Now()

	s1 := models.ClientSession{
		SessionID:     "test-1",
		UserID:        1,
		Username:      "testuser",
		ClientIP:      "127.0.0.1",
		ServerID:      "device-1",
		ActiveTasks:   2,
		LastHeartbeat: now,
		Status:        "online",
		ConnectedAt:   now,
	}
	sm.RegisterOrUpdate(s1)

	online := sm.ListOnline()
	if len(online) != 1 {
		t.Errorf("expected 1 online session, got %d", len(online))
	}
	if online[0].SessionID != "test-1" {
		t.Errorf("expected session ID test-1, got %s", online[0].SessionID)
	}

	s1Updated := models.ClientSession{
		SessionID:     "test-1",
		ActiveTasks:   5,
		LastHeartbeat: now.Add(5 * time.Minute),
		Status:        "online",
	}
	sm.RegisterOrUpdate(s1Updated)

	online = sm.ListOnline()
	if online[0].ActiveTasks != 5 {
		t.Errorf("expected 5 active tasks, got %d", online[0].ActiveTasks)
	}
}

func TestListOnline(t *testing.T) {
	sm := NewSessionManager()
	now := time.Now()

	s1 := models.ClientSession{SessionID: "test-1", Status: "online", LastHeartbeat: now}
	s2 := models.ClientSession{SessionID: "test-2", Status: "offline", LastHeartbeat: now}
	s3 := models.ClientSession{SessionID: "test-3", Status: "online", LastHeartbeat: now}

	sm.RegisterOrUpdate(s1)
	sm.RegisterOrUpdate(s2)
	sm.RegisterOrUpdate(s3)

	online := sm.ListOnline()
	if len(online) != 2 {
		t.Errorf("expected 2 online sessions, got %d", len(online))
	}
}

func TestCleanupStale(t *testing.T) {
	sm := NewSessionManager()
	now := time.Now()

	s1 := models.ClientSession{SessionID: "test-1", Status: "online", LastHeartbeat: now.Add(-30 * time.Minute)}
	s2 := models.ClientSession{SessionID: "test-2", Status: "online", LastHeartbeat: now}
	s3 := models.ClientSession{SessionID: "test-3", Status: "offline", LastHeartbeat: now.Add(-60 * time.Minute)}

	sm.RegisterOrUpdate(s1)
	sm.RegisterOrUpdate(s2)
	sm.RegisterOrUpdate(s3)

	sm.CleanupStale(15 * time.Minute)

	online := sm.ListOnline()
	if len(online) != 1 {
		t.Errorf("expected 1 online session, got %d", len(online))
	}
	if online[0].SessionID != "test-2" {
		t.Errorf("expected test-2 to be online, got %s", online[0].SessionID)
	}
}

func TestConcurrentAccess(t *testing.T) {
	sm := NewSessionManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s := models.ClientSession{
				SessionID:     string(rune('0' + id)),
				UserID:        uint(id),
				LastHeartbeat: time.Now(),
				Status:        "online",
			}
			sm.RegisterOrUpdate(s)
		}(i)
	}

	wg.Wait()

	online := sm.ListOnline()
	if len(online) != 100 {
		t.Errorf("expected 100 online sessions, got %d", len(online))
	}
}

func TestStartCleanupLoop(t *testing.T) {
	sm := NewSessionManager()
	ctx, cancel := context.WithCancel(context.Background())

	go sm.StartCleanupLoop(ctx, 100*time.Millisecond, 200*time.Millisecond)

	s := models.ClientSession{
		SessionID:     "test",
		Status:        "online",
		LastHeartbeat: time.Now().Add(-300 * time.Millisecond),
	}
	sm.RegisterOrUpdate(s)

	time.Sleep(300 * time.Millisecond)

	online := sm.ListOnline()
	if len(online) != 0 {
		t.Errorf("expected 0 online sessions after cleanup, got %d", len(online))
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

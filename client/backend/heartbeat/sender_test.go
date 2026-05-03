package heartbeat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"zcopy-client-backend/auth"
	"zcopy-client-backend/proxy"
)

var _ auth.TokenProvider = (*mockTokenProvider)(nil)

type mockTokenProvider struct {
	mu     sync.Mutex
	tokens map[string]string
}

func (m *mockTokenProvider) GetToken(serverID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	token, ok := m.tokens[serverID]
	return token, ok
}

func (m *mockTokenProvider) SetToken(serverID string, token string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.tokens == nil {
		m.tokens = make(map[string]string)
	}
	m.tokens[serverID] = token
}

func (m *mockTokenProvider) RemoveToken(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, serverID)
}

func TestHeartbeatSender_SendHeartbeat(t *testing.T) {
	var received bool
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received = true
		mu.Unlock()

		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/client/heartbeat" {
			t.Errorf("Expected path /api/v1/client/heartbeat, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		var body map[string]int
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		if body["active_tasks"] != 0 {
			t.Errorf("Expected active_tasks=0, got %d", body["active_tasks"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokenProvider := &mockTokenProvider{tokens: map[string]string{"test-server": "test-token"}}
	proxyFactory := func(serverID string) (*proxy.HTTPRemoteClient, error) {
		return proxy.NewHTTPRemoteClient(server.Client(), server.URL), nil
	}

	sender := NewHeartbeatSender(tokenProvider, proxyFactory)
	sender.sendHeartbeat("test-server", server.Listener.Addr().String())

	mu.Lock()
	if !received {
		t.Errorf("Expected heartbeat to be sent")
	}
	mu.Unlock()
}

func TestHeartbeatSender_StartStopForServer(t *testing.T) {
	var received int
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tokenProvider := &mockTokenProvider{tokens: map[string]string{"test-server": "test-token"}}
	proxyFactory := func(serverID string) (*proxy.HTTPRemoteClient, error) {
		return proxy.NewHTTPRemoteClient(server.Client(), server.URL), nil
	}

	sender := NewHeartbeatSender(tokenProvider, proxyFactory)

	ctx, cancel := context.WithCancel(context.Background())
	sender.mu.Lock()
	sender.senders["test-server"] = cancel
	sender.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sender.sendHeartbeat("test-server", server.Listener.Addr().String())
			case <-ctx.Done():
				return
			}
		}
	}()

	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	count := received
	mu.Unlock()

	if count < 2 {
		t.Errorf("Expected at least 2 heartbeats, got %d", count)
	}

	sender.StopForServer("test-server")

	prevCount := count
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count = received
	mu.Unlock()

	if count > prevCount {
		t.Errorf("Expected no more heartbeats after stop, but count increased from %d to %d", prevCount, count)
	}
}

func TestHeartbeatSender_StopAll(t *testing.T) {
	var received1, received2 int
	var mu1, mu2 sync.Mutex

	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu1.Lock()
		received1++
		mu1.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu2.Lock()
		received2++
		mu2.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server2.Close()

	tokenProvider := &mockTokenProvider{tokens: map[string]string{
		"server1": "token1",
		"server2": "token2",
	}}

	proxyFactory := func(serverID string) (*proxy.HTTPRemoteClient, error) {
		if serverID == "server1" {
			return proxy.NewHTTPRemoteClient(server1.Client(), server1.URL), nil
		}
		return proxy.NewHTTPRemoteClient(server2.Client(), server2.URL), nil
	}

	sender := NewHeartbeatSender(tokenProvider, proxyFactory)

	sender.mu.Lock()

	ctx1, cancel1 := context.WithCancel(context.Background())
	sender.senders["server1"] = cancel1

	ctx2, cancel2 := context.WithCancel(context.Background())
	sender.senders["server2"] = cancel2

	sender.mu.Unlock()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sender.sendHeartbeat("server1", server1.Listener.Addr().String())
			case <-ctx1.Done():
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sender.sendHeartbeat("server2", server2.Listener.Addr().String())
			case <-ctx2.Done():
				return
			}
		}
	}()

	time.Sleep(250 * time.Millisecond)

	mu1.Lock()
	count1 := received1
	mu1.Unlock()
	mu2.Lock()
	count2 := received2
	mu2.Unlock()

	if count1 < 2 {
		t.Errorf("Server1 expected at least 2 heartbeats, got %d", count1)
	}
	if count2 < 2 {
		t.Errorf("Server2 expected at least 2 heartbeats, got %d", count2)
	}

	sender.StopAll()

	prev1, prev2 := count1, count2
	time.Sleep(200 * time.Millisecond)

	mu1.Lock()
	count1 = received1
	mu1.Unlock()
	mu2.Lock()
	count2 = received2
	mu2.Unlock()

	if count1 > prev1 {
		t.Errorf("Server1 got unexpected heartbeats after StopAll")
	}
	if count2 > prev2 {
		t.Errorf("Server2 got unexpected heartbeats after StopAll")
	}
}

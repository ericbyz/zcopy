package proxy

import (
	"testing"
)

// MockTokenProvider is a mock implementation of auth.TokenProvider
type MockTokenProvider struct {
	tokens map[string]string
}

func NewMockTokenProvider() *MockTokenProvider {
	return &MockTokenProvider{tokens: make(map[string]string)}
}

func (m *MockTokenProvider) GetToken(serverID string) (string, bool) {
	token, ok := m.tokens[serverID]
	return token, ok
}

func (m *MockTokenProvider) SetToken(serverID string, token string) {
	m.tokens[serverID] = token
}

func (m *MockTokenProvider) RemoveToken(serverID string) {
	delete(m.tokens, serverID)
}

func TestMultiServerProxy_AddServerAndGetClient(t *testing.T) {
	tp := NewMockTokenProvider()
	proxy := NewMultiServerProxy(tp)

	proxy.AddServer("server1", "https://server1.example.com")
	proxy.AddServer("server2", "https://server2.example.com")

	client1, err := proxy.GetClient("server1")
	if err != nil {
		t.Fatalf("GetClient(server1) failed: %v", err)
	}
	if client1.BaseURL != "https://server1.example.com" {
		t.Errorf("Expected BaseURL 'https://server1.example.com', got '%s'", client1.BaseURL)
	}

	client2, err := proxy.GetClient("server2")
	if err != nil {
		t.Fatalf("GetClient(server2) failed: %v", err)
	}
	if client2.BaseURL != "https://server2.example.com" {
		t.Errorf("Expected BaseURL 'https://server2.example.com', got '%s'", client2.BaseURL)
	}
}

func TestMultiServerProxy_GetClientNonExistent(t *testing.T) {
	tp := NewMockTokenProvider()
	proxy := NewMultiServerProxy(tp)

	_, err := proxy.GetClient("nonexistent")
	if err == nil {
		t.Fatal("Expected error for GetClient(nonexistent), got nil")
	}
	if err != ErrServerNotRegistered {
		t.Fatalf("Expected ErrServerNotRegistered, got %v", err)
	}
}

func TestMultiServerProxy_RemoveServer(t *testing.T) {
	tp := NewMockTokenProvider()
	proxy := NewMultiServerProxy(tp)

	proxy.AddServer("server1", "https://server1.example.com")
	if !proxy.HasServer("server1") {
		t.Fatal("Expected HasServer(server1) to return true after AddServer")
	}

	proxy.RemoveServer("server1")
	if proxy.HasServer("server1") {
		t.Fatal("Expected HasServer(server1) to return false after RemoveServer")
	}

	_, err := proxy.GetClient("server1")
	if err == nil {
		t.Fatal("Expected error for GetClient(server1) after RemoveServer")
	}
}

func TestMultiServerProxy_ListServerIDs(t *testing.T) {
	tp := NewMockTokenProvider()
	proxy := NewMultiServerProxy(tp)

	proxy.AddServer("server1", "https://server1.example.com")
	proxy.AddServer("server2", "https://server2.example.com")

	ids := proxy.ListServerIDs()
	if len(ids) != 2 {
		t.Fatalf("Expected 2 server IDs, got %d", len(ids))
	}

	seen := make(map[string]bool)
	for _, id := range ids {
		seen[id] = true
	}
	if !seen["server1"] || !seen["server2"] {
		t.Fatal("Expected server1 and server2 in ListServerIDs")
	}
}

func TestMultiServerProxy_HasServer(t *testing.T) {
	tp := NewMockTokenProvider()
	proxy := NewMultiServerProxy(tp)

	if proxy.HasServer("server1") {
		t.Fatal("Expected HasServer(server1) to return false before AddServer")
	}

	proxy.AddServer("server1", "https://server1.example.com")
	if !proxy.HasServer("server1") {
		t.Fatal("Expected HasServer(server1) to return true after AddServer")
	}
}

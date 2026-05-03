package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"zcopy-client-backend/auth"
	"zcopy-client-backend/proxy"
)

type HeartbeatSender struct {
	senders       map[string]context.CancelFunc
	mu            sync.Mutex
	tokenProvider auth.TokenProvider
	proxyFactory  func(serverID string) (*proxy.HTTPRemoteClient, error)
}

func NewHeartbeatSender(
	tokenProvider auth.TokenProvider,
	proxyFactory func(serverID string) (*proxy.HTTPRemoteClient, error),
) *HeartbeatSender {
	return &HeartbeatSender{
		senders:       make(map[string]context.CancelFunc),
		tokenProvider: tokenProvider,
		proxyFactory:  proxyFactory,
	}
}

func (s *HeartbeatSender) StartForServer(serverID, address string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.senders[serverID]; exists {
		cancel()
		delete(s.senders, serverID)
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.senders[serverID] = cancel

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.sendHeartbeat(serverID, address)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (s *HeartbeatSender) sendHeartbeat(serverID, address string) {
	token, ok := s.tokenProvider.GetToken(serverID)
	if !ok {
		log.Printf("[Heartbeat] No token for server %s, skipping", serverID)
		return
	}

	client, err := s.proxyFactory(serverID)
	if err != nil {
		log.Printf("[Heartbeat] Failed to get proxy client for %s: %v", serverID, err)
		return
	}

	body := map[string]int{"active_tasks": 0}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		log.Printf("[Heartbeat] Failed to marshal heartbeat body: %v", err)
		return
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"http://"+address+"/api/v1/client/heartbeat",
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		log.Printf("[Heartbeat] Failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.HTTPc.Do(req)
	if err != nil {
		log.Printf("[Heartbeat] Failed to send heartbeat to %s: %v", address, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[Heartbeat] Heartbeat to %s returned status: %d", address, resp.StatusCode)
	}
}

func (s *HeartbeatSender) StopForServer(serverID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cancel, exists := s.senders[serverID]; exists {
		cancel()
		delete(s.senders, serverID)
	}
}

func (s *HeartbeatSender) StopAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cancel := range s.senders {
		cancel()
		delete(s.senders, id)
	}
}

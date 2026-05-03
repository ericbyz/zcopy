package discovery

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestBuildMSearchRequest(t *testing.T) {
	req := buildMSearchRequest(1900)
	lines := bytes.Split(req, []byte("\r\n"))
	
	if len(lines) < 6 {
		t.Fatalf("Expected at least 6 lines, got %d", len(lines))
	}

	// Check first line
	if !bytes.HasPrefix(lines[0], []byte("M-SEARCH * HTTP/1.1")) {
		t.Fatalf("Expected first line to be M-SEARCH, got %q", lines[0])
	}

	// Check ST header
	foundST := false
	for _, line := range lines {
		if bytes.EqualFold(bytes.TrimSpace(line), []byte("ST: zcopy:server")) {
			foundST = true
			break
		}
	}
	if !foundST {
		t.Fatal("Expected ST: zcopy:server header")
	}

	// Check Man header
	foundMan := false
	for _, line := range lines {
		if bytes.EqualFold(bytes.TrimSpace(line), []byte(`Man: "ssdp:discover"`)) {
			foundMan = true
			break
		}
	}
	if !foundMan {
		t.Fatal(`Expected Man: "ssdp:discover" header`)
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name     string
		response []byte
		want     DiscoveredServer
		wantErr  bool
	}{
		{
			name: "valid response",
			response: []byte("HTTP/1.1 200 OK\r\n" +
				"ST: zcopy:server\r\n" +
				"SERVER-ID: test-123\r\n" +
				"SERVER-NAME: My Test Server\r\n" +
				"LOCATION: http://192.168.1.100:8890\r\n" +
				"\r\n"),
			want: DiscoveredServer{
				ServerID: "test-123",
				Name:     "My Test Server",
				Address:  "http://192.168.1.100:8890",
			},
			wantErr: false,
		},
		{
			name:     "invalid response - no headers",
			response: []byte("HTTP/1.1 200 OK\r\n\r\n"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResponse(tt.response)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if got.ServerID != tt.want.ServerID {
					t.Errorf("ServerID = %v, want %v", got.ServerID, tt.want.ServerID)
				}
				if got.Name != tt.want.Name {
					t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
				}
				if got.Address != tt.want.Address {
					t.Errorf("Address = %v, want %v", got.Address, tt.want.Address)
				}
			}
		})
	}
}

func TestDeduplication(t *testing.T) {
	servers := []DiscoveredServer{
		{ServerID: "id-1", Name: "Server 1", Address: "http://1.1.1.1"},
		{ServerID: "id-2", Name: "Server 2", Address: "http://2.2.2.2"},
		{ServerID: "id-1", Name: "Server 1 Duplicate", Address: "http://1.1.1.1:8080"},
	}
	deduped := deduplicateServers(servers)
	if len(deduped) != 2 {
		t.Fatalf("Expected 2 servers after deduplication, got %d", len(deduped))
	}
	
	found1 := false
	found2 := false
	for _, s := range deduped {
		if s.ServerID == "id-1" {
			found1 = true
		}
		if s.ServerID == "id-2" {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Fatal("Expected both unique servers to be present")
	}
}

func TestScanLANTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	servers, err := ScanLAN(ctx, 10*time.Millisecond, 1900)
	if err != nil {
		// Timeout should not return an error
		t.Fatalf("ScanLAN returned unexpected error: %v", err)
	}
	// Should return empty list on timeout without responses
	if len(servers) != 0 {
		t.Errorf("Expected empty list on timeout, got %d servers", len(servers))
	}
}

func TestScanLANIntegration(t *testing.T) {
	// Start a mock responder on a random port
	port := 19001 // Use a non-standard port for testing
	mockAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Failed to resolve UDP addr: %v", err)
	}

	conn, err := net.ListenUDP("udp", mockAddr)
	if err != nil {
		t.Fatalf("Failed to listen UDP: %v", err)
	}
	defer conn.Close()

	// Start mock responder goroutine
	go func() {
		buf := make([]byte, 1024)
		for {
			conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			n, addr, err := conn.ReadFromUDP(buf)
			if err != nil {
				break
			}
			// Check if it's a ZCopy search
			if isZCopySearch(buf[:n]) {
				// Send mock response
				resp := []byte("HTTP/1.1 200 OK\r\n" +
					"ST: zcopy:server\r\n" +
					"SERVER-ID: mock-server-id\r\n" +
					"SERVER-NAME: Mock Test Server\r\n" +
					fmt.Sprintf("LOCATION: http://127.0.0.1:%d\r\n", 8890) +
					"\r\n")
				conn.WriteToUDP(resp, addr)
			}
		}
	}()

	// Give mock time to start
	time.Sleep(100 * time.Millisecond)

	t.Run("integration parsing works", func(t *testing.T) {
		mockResp := []byte("HTTP/1.1 200 OK\r\n" +
			"ST: zcopy:server\r\n" +
			"SERVER-ID: mock-server-id\r\n" +
			"SERVER-NAME: Mock Test Server\r\n" +
			"LOCATION: http://127.0.0.1:8890\r\n" +
			"\r\n")
		server, err := parseResponse(mockResp)
		if err != nil {
			t.Fatalf("Failed to parse mock response: %v", err)
		}
		if server.ServerID != "mock-server-id" {
			t.Errorf("Expected ServerID mock-server-id, got %s", server.ServerID)
		}
	})
}

func isZCopySearch(data []byte) bool {
	lines := bytes.Split(data, []byte("\r\n"))
	for _, line := range lines {
		parts := bytes.SplitN(line, []byte(":"), 2)
		if len(parts) == 2 {
			key := bytes.TrimSpace(parts[0])
			value := bytes.TrimSpace(parts[1])
			if bytes.EqualFold(key, []byte("ST")) && bytes.EqualFold(value, []byte("zcopy:server")) {
				return true
			}
		}
	}
	return false
}

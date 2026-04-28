package discovery

import (
	"bytes"
	"net"
	"testing"
	"time"

	"zcopy-server-backend/models"
)

func TestResponderLifecycle(t *testing.T) {
	si := &models.ServerInfo{
		UUID:    "test-uuid-1234",
		Name:    "Test Server",
		Address: "localhost:8890",
	}

	responder := NewResponder(si, 0)
	if responder == nil {
		t.Fatal("NewResponder returned nil")
	}

	err := responder.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify server is listening
	time.Sleep(100 * time.Millisecond)

	err = responder.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestZCopyDiscoveryResponse(t *testing.T) {
	si := &models.ServerInfo{
		UUID:    "test-uuid-5678",
		Name:    "Test Server",
		Address: "localhost:8890",
	}

	responder := NewResponder(si, 0)
	err := responder.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer responder.Stop()

	time.Sleep(100 * time.Millisecond)

	// Get the actual port
	localAddr := responder.conn.LocalAddr().(*net.UDPAddr)

	// Send M-SEARCH request
	clientConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		t.Fatalf("DialUDP failed: %v", err)
	}
	defer clientConn.Close()

	searchRequest := []byte("M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 1\r\n" +
		"ST: zcopy:server\r\n" +
		"\r\n")

	_, err = clientConn.Write(searchRequest)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait for response
	buffer := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := clientConn.ReadFromUDP(buffer)
	if err != nil {
		t.Fatalf("ReadFromUDP failed: %v", err)
	}

	response := string(buffer[:n])

	// Verify response content
	if !bytes.Contains([]byte(response), []byte("HTTP/1.1 200 OK")) {
		t.Error("Response missing HTTP 200 OK")
	}
	if !bytes.Contains([]byte(response), []byte("SERVER-ID: test-uuid-5678")) {
		t.Error("Response missing SERVER-ID")
	}
	if !bytes.Contains([]byte(response), []byte("SERVER-NAME: Test Server")) {
		t.Error("Response missing SERVER-NAME")
	}
	if !bytes.Contains([]byte(response), []byte("LOCATION: http://localhost:8890")) {
		t.Error("Response missing LOCATION")
	}
}

func TestNonZCopyDiscoveryIgnored(t *testing.T) {
	si := &models.ServerInfo{
		UUID:    "test-uuid-9012",
		Name:    "Test Server",
		Address: "localhost:8890",
	}

	responder := NewResponder(si, 0)
	err := responder.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer responder.Stop()

	time.Sleep(100 * time.Millisecond)

	// Get the actual port
	localAddr := responder.conn.LocalAddr().(*net.UDPAddr)

	// Send non-zcopy M-SEARCH request
	clientConn, err := net.DialUDP("udp", nil, localAddr)
	if err != nil {
		t.Fatalf("DialUDP failed: %v", err)
	}
	defer clientConn.Close()

	searchRequest := []byte("M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 1\r\n" +
		"ST: upnp:rootdevice\r\n" +
		"\r\n")

	_, err = clientConn.Write(searchRequest)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Wait and check no response
	buffer := make([]byte, 1024)
	clientConn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err = clientConn.ReadFromUDP(buffer)
	if err == nil {
		t.Fatal("Received response when should have ignored")
	}
}

package discovery

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

type DiscoveredServer struct {
	ServerID string `json:"serverId"`
	Name     string `json:"name"`
	Address  string `json:"address"`
}

func ScanLAN(ctx context.Context, timeout time.Duration, port int) ([]DiscoveredServer, error) {
	if port == 0 {
		port = 1900
	}

	conn, err := net.ListenUDP("udp", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(timeout))

	req := buildMSearchRequest(port)
	broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", port))
	if err == nil {
		_, _ = conn.WriteToUDP(req, broadcastAddr)
	}

	multicastAddr, errMulticast := net.ResolveUDPAddr("udp", fmt.Sprintf("239.255.255.250:%d", port))
	if errMulticast == nil {
		_, _ = conn.WriteToUDP(req, multicastAddr)
	}


	// Collect responses
	var servers []DiscoveredServer
	buf := make([]byte, 1024)

	for {
		select {
		case <-ctx.Done():
			return deduplicateServers(servers), nil
		default:
			conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return deduplicateServers(servers), nil
			}

			server, err := parseResponse(buf[:n])
			if err == nil {
				servers = append(servers, server)
			}
		}
	}
}

func buildMSearchRequest(port int) []byte {
	var buf bytes.Buffer
	buf.WriteString("M-SEARCH * HTTP/1.1\r\n")
	buf.WriteString(fmt.Sprintf("Host: 239.255.255.250:%d\r\n", port))
	buf.WriteString("ST: zcopy:server\r\n")
	buf.WriteString("Man: \"ssdp:discover\"\r\n")
	buf.WriteString("MX: 3\r\n")
	buf.WriteString("\r\n")
	return buf.Bytes()
}

func parseResponse(data []byte) (DiscoveredServer, error) {
	var server DiscoveredServer
	lines := strings.Split(string(data), "\r\n")
	foundID := false
	foundName := false
	foundLocation := false

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			switch {
			case strings.EqualFold(key, "SERVER-ID"):
				server.ServerID = value
				foundID = true
			case strings.EqualFold(key, "SERVER-NAME"):
				server.Name = value
				foundName = true
			case strings.EqualFold(key, "LOCATION"):
				server.Address = value
				foundLocation = true
			}
		}
	}

	if !foundID || !foundName || !foundLocation {
		return DiscoveredServer{}, fmt.Errorf("missing required headers")
	}

	return server, nil
}

func deduplicateServers(servers []DiscoveredServer) []DiscoveredServer {
	seen := make(map[string]bool)
	var result []DiscoveredServer
	for _, s := range servers {
		if !seen[s.ServerID] {
			seen[s.ServerID] = true
			result = append(result, s)
		}
	}
	return result
}

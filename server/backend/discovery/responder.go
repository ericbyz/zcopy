package discovery

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"zcopy-server-backend/models"
)

type Responder struct {
	conn       *net.UDPConn
	serverInfo *models.ServerInfo
	port       int
	ctx        context.Context
	cancel     context.CancelFunc
}

func NewResponder(serverInfo *models.ServerInfo, port int) *Responder {
	if port == 0 {
		port = 1900
	}
	return &Responder{
		serverInfo: serverInfo,
		port:       port,
	}
}

func (r *Responder) Start() error {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}

	r.ctx, r.cancel = context.WithCancel(context.Background())
	r.conn = conn

	go r.listenLoop()

	return nil
}

func (r *Responder) listenLoop() {
	buffer := make([]byte, 1024)

	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			r.conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, remoteAddr, err := r.conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			if r.isZCopySearch(buffer[:n]) {
				response := r.buildResponse()
				r.conn.WriteToUDP(response, remoteAddr)
			}
		}
	}
}

func (r *Responder) isZCopySearch(data []byte) bool {
	lines := strings.Split(string(data), "\r\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if strings.EqualFold(key, "ST") && strings.EqualFold(value, "zcopy:server") {
				return true
			}
		}
	}
	return false
}

func (r *Responder) buildResponse() []byte {
	var buf bytes.Buffer
	buf.WriteString("HTTP/1.1 200 OK\r\n")
	buf.WriteString("ST: zcopy:server\r\n")
	buf.WriteString(fmt.Sprintf("SERVER-ID: %s\r\n", r.serverInfo.UUID))
	buf.WriteString(fmt.Sprintf("SERVER-NAME: %s\r\n", r.serverInfo.Name))
	buf.WriteString(fmt.Sprintf("LOCATION: http://%s\r\n", r.serverInfo.Address))
	buf.WriteString("\r\n")
	return buf.Bytes()
}

func (r *Responder) Stop() error {
	if r.cancel != nil {
		r.cancel()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

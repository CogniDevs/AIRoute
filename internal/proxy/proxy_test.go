package proxy

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"airoute/internal/domain"
)

func TestProxyServer_Connect(t *testing.T) {
	// 1. Setup mock backend server on an arbitrary port
	mockListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind mock listener: %v", err)
	}
	defer mockListener.Close()
	mockPort := mockListener.Addr().(*net.TCPAddr).Port

	go func() {
		for {
			conn, err := mockListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				scanner := bufio.NewScanner(c)
				if scanner.Scan() {
					line := scanner.Text()
					if strings.HasPrefix(line, "PING") {
						_, _ = c.Write([]byte("PONG\n"))
					}
				}
			}(conn)
		}
	}()

	// 2. Start proxy server on random available port
	srv := NewServer()
	err = srv.Start("127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start proxy server: %v", err)
	}
	defer srv.Stop()

	if !srv.IsRunning() {
		t.Fatal("expected proxy to be running")
	}

	proxyAddr := srv.GetAddr()

	// 3. Configure mock route: gemini.google.com -> 127.0.0.1 (our mock port is contacted)
	srv.UpdateRoutes(map[domain.ServiceTarget]domain.ServiceRoute{
		domain.TargetGemini: {
			Target:  domain.TargetGemini,
			Enabled: true,
			IP:      "127.0.0.1",
		},
	})

	// 4. Dial proxy and send CONNECT gemini.google.com:<mockPort>
	clientConn, err := net.DialTimeout("tcp", proxyAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to proxy: %v", err)
	}
	defer clientConn.Close()

	req := fmt.Sprintf("CONNECT gemini.google.com:%d HTTP/1.1\r\nHost: gemini.google.com:%d\r\n\r\n", mockPort, mockPort)
	if _, err := clientConn.Write([]byte(req)); err != nil {
		t.Fatalf("failed to write CONNECT: %v", err)
	}

	reader := bufio.NewReader(clientConn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatalf("failed to read CONNECT response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from proxy, got %d", resp.StatusCode)
	}

	// 5. Test payload tunneling
	if _, err := clientConn.Write([]byte("PING\n")); err != nil {
		t.Fatalf("failed to write payload: %v", err)
	}

	reply, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read reply from tunnel: %v", err)
	}
	if strings.TrimSpace(reply) != "PONG" {
		t.Fatalf("expected PONG, got %q", reply)
	}
}

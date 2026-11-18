package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"airoute/internal/domain"
)

// Server is an embedded local HTTP CONNECT proxy for routing AI traffic without modifying hosts.
type Server struct {
	mu          sync.RWMutex
	listener    net.Listener
	httpServer  *http.Server
	addr        string
	running     bool
	domainToIP  map[string]string // domain -> routed gateway IP
}

// NewServer creates a new local proxy server instance.
func NewServer() *Server {
	return &Server{
		domainToIP: make(map[string]string),
	}
}

// UpdateRoutes updates the in-memory domain-to-IP routing table.
func (s *Server) UpdateRoutes(routes map[domain.ServiceTarget]domain.ServiceRoute) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.domainToIP = make(map[string]string)
	for target, route := range routes {
		if !route.Enabled || strings.TrimSpace(route.IP) == "" {
			continue
		}
		for _, d := range domain.TargetDomains[target] {
			s.domainToIP[strings.ToLower(d)] = route.IP
		}
	}
}

// IsRunning reports whether the proxy is actively serving requests.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetAddr returns the listening address of the proxy.
func (s *Server) GetAddr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

// Start launches the proxy on the requested address (e.g. "127.0.0.1:2080").
func (s *Server) Start(addr string) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	if addr == "" {
		addr = "127.0.0.1:2080"
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("proxy listen %s: %w", addr, err)
	}

	s.listener = ln
	s.addr = ln.Addr().String()
	s.running = true

	s.httpServer = &http.Server{
		Handler:      http.HandlerFunc(s.handleHTTP),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
	s.mu.Unlock()

	go func() {
		_ = s.httpServer.Serve(ln)
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()

	return nil
}

// Stop shuts down the proxy server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running || s.httpServer == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := s.httpServer.Shutdown(ctx)
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.running = false
	return err
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
	} else {
		s.handleStandardHTTP(w, r)
	}
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	host, port, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
		port = "443"
	}

	s.mu.RLock()
	routedIP := s.domainToIP[strings.ToLower(host)]
	s.mu.RUnlock()

	targetAddr := net.JoinHostPort(host, port)
	if routedIP != "" {
		targetAddr = net.JoinHostPort(routedIP, port)
	}

	targetConn, err := net.DialTimeout("tcp", targetAddr, 5*time.Second)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect to gateway (%v)", err), http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		targetConn.Close()
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		targetConn.Close()
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	// Signal to client that tunnel is ready
	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	// Bidirectional tunnel
	go func() {
		defer targetConn.Close()
		defer clientConn.Close()
		_, _ = io.Copy(targetConn, clientConn)
	}()

	go func() {
		defer targetConn.Close()
		defer clientConn.Close()
		_, _ = io.Copy(clientConn, targetConn)
	}()
}

func (s *Server) handleStandardHTTP(w http.ResponseWriter, r *http.Request) {
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, r.RequestURI, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for k, vv := range r.Header {
		for _, v := range vv {
			outReq.Header.Add(k, v)
		}
	}

	transport := http.DefaultTransport
	resp, err := transport.RoundTrip(outReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

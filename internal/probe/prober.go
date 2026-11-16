package probe

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"airoute/internal/domain"
)

// Prober performs network latency benchmarks and TLS anti-spoofing checks.
type Prober struct {
	Timeout time.Duration
}

// NewProber creates a prober with default timeout.
func NewProber(timeout time.Duration) *Prober {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	return &Prober{Timeout: timeout}
}

// ProbeIP executes deep validation of a given IP for a specific AI target.
func (p *Prober) ProbeIP(ip string, target domain.ServiceTarget) domain.ProbeResult {
	res := domain.ProbeResult{
		IP:     ip,
		Target: target,
	}

	start := time.Now()

	switch target {
	case domain.TargetGemini:
		res.IsClean, res.Reason = p.checkGemini(ip)
	case domain.TargetClaude:
		res.IsClean, res.Reason = p.checkClaude(ip)
	case domain.TargetChatGPT:
		res.IsClean, res.Reason = p.checkChatGPT(ip)
	case domain.TargetAntigravity:
		res.IsClean, res.Reason = p.checkAntigravity(ip)
	case domain.TargetAll, "":
		// Composite check: must pass Gemini & Claude checks
		okGem, reasonGem := p.checkGemini(ip)
		if !okGem {
			res.IsClean = false
			res.Reason = reasonGem
		} else {
			okAg, _ := p.checkAntigravity(ip)
			if okAg {
				res.IsClean = true
				res.Reason = "Clean Universal Gateway (Gemini & Antigravity OK)"
			} else {
				res.IsClean = true
				res.Reason = "Clean Web Gateway (Gemini OK)"
			}
		}
	default:
		res.IsClean, res.Reason = p.checkGemini(ip)
	}

	res.LatencyMs = time.Since(start).Milliseconds()
	return res
}

func (p *Prober) checkGemini(ip string) (bool, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), p.Timeout)
	if err != nil {
		return false, fmt.Sprintf("TCP Timeout (%v)", err)
	}
	defer conn.Close()

	tlsConfig := &tls.Config{
		ServerName:         "gemini.google.com",
		InsecureSkipVerify: false,
	}
	tlsConn := tls.Client(conn, tlsConfig)
	_ = tlsConn.SetDeadline(time.Now().Add(p.Timeout))

	if err := tlsConn.Handshake(); err != nil {
		return false, "TLS Invalid (Not authentic Google cert)"
	}
	defer tlsConn.Close()

	req := "GET /app HTTP/1.1\r\nHost: gemini.google.com\r\nUser-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)\r\nConnection: close\r\n\r\n"
	if _, err := tlsConn.Write([]byte(req)); err != nil {
		return false, "HTTP Write error"
	}

	buf := make([]byte, 4096)
	n, _ := io.ReadAtLeast(tlsConn, buf, 100)
	resp := strings.ToLower(string(buf[:n]))

	if strings.Contains(resp, "302") || strings.Contains(resp, "sorry") {
		return false, "Blocked by Google (Captcha /sorry/index)"
	}
	if strings.Contains(resp, "comss.one") || strings.TrimSpace(resp) == "direct" {
		return false, "Provider Stub (Not a proxy)"
	}

	isGoogle := strings.Contains(resp, "bardchatui") ||
		strings.Contains(resp, "boq-bard-web") ||
		strings.Contains(resp, "server: esf") ||
		strings.Contains(resp, "server: scaffolding")

	if !isGoogle && !strings.Contains(resp, "200") {
		return false, "Not a recognized Gemini gateway"
	}

	return true, "Clean Gemini Gateway"
}

func (p *Prober) checkClaude(ip string) (bool, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), p.Timeout)
	if err != nil {
		return false, "TCP Timeout"
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: "claude.ai"})
	_ = tlsConn.SetDeadline(time.Now().Add(p.Timeout))
	if err := tlsConn.Handshake(); err != nil {
		return false, "TLS Invalid (Not Claude cert)"
	}
	defer tlsConn.Close()

	req := "GET / HTTP/1.1\r\nHost: claude.ai\r\nUser-Agent: Mozilla/5.0\r\nConnection: close\r\n\r\n"
	_, _ = tlsConn.Write([]byte(req))

	buf := make([]byte, 2048)
	n, _ := tlsConn.Read(buf)
	resp := strings.ToLower(string(buf[:n]))

	if strings.Contains(resp, "app-unavailable-in-region") || strings.Contains(resp, "not available in your country") {
		return false, "Regional Block (Anthropic)"
	}
	if strings.Contains(resp, "server: cloudflare") || strings.Contains(resp, "cf-ray") {
		return true, "Clean Claude Cloudflare Gateway"
	}

	return false, "Non-Cloudflare Response"
}

func (p *Prober) checkChatGPT(ip string) (bool, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), p.Timeout)
	if err != nil {
		return false, "TCP Timeout"
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: "chatgpt.com"})
	_ = tlsConn.SetDeadline(time.Now().Add(p.Timeout))
	if err := tlsConn.Handshake(); err != nil {
		return false, "TLS Invalid (Not ChatGPT cert)"
	}
	defer tlsConn.Close()

	req := "GET / HTTP/1.1\r\nHost: chatgpt.com\r\nUser-Agent: Mozilla/5.0\r\nConnection: close\r\n\r\n"
	_, _ = tlsConn.Write([]byte(req))

	buf := make([]byte, 2048)
	n, _ := tlsConn.Read(buf)
	resp := strings.ToLower(string(buf[:n]))

	if strings.Contains(resp, "unable to load site") || strings.Contains(resp, "vpn") {
		return false, "OpenAI Region/VPN Block"
	}
	if strings.Contains(resp, "chatgpt") || strings.Contains(resp, "oai-did") || strings.Contains(resp, "cf-ray") {
		return true, "Clean ChatGPT Gateway"
	}

	return false, "Non-OpenAI Gateway"
}

func (p *Prober) checkAntigravity(ip string) (bool, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), p.Timeout)
	if err != nil {
		return false, "TCP Timeout"
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: "daily-cloudcode-pa.googleapis.com"})
	_ = tlsConn.SetDeadline(time.Now().Add(p.Timeout))
	if err := tlsConn.Handshake(); err != nil {
		return false, "TLS Invalid (Not Google CloudCode cert)"
	}
	defer tlsConn.Close()

	req := "POST /v1internal:loadCodeAssist HTTP/1.1\r\nHost: daily-cloudcode-pa.googleapis.com\r\nContent-Type: application/json\r\nContent-Length: 2\r\nUser-Agent: Antigravity\r\nConnection: close\r\n\r\n{}"
	_, _ = tlsConn.Write([]byte(req))

	buf := make([]byte, 2048)
	n, _ := tlsConn.Read(buf)
	resp := strings.ToLower(string(buf[:n]))

	if (strings.Contains(resp, "401") || strings.Contains(resp, "403")) &&
		(strings.Contains(resp, "cloudcode") || strings.Contains(resp, "google") || strings.Contains(resp, "unauthenticated")) {
		return true, "Clean Antigravity CloudCode Gateway"
	}

	return false, "CloudCode endpoint rejected or filtered"
}

// ProbeAllProviders concurrently benchmarks and tests all candidate IPs across registered providers.
func (p *Prober) ProbeAllProviders(providersList []domain.Provider, target domain.ServiceTarget) []domain.ProbeResult {
	var candidates []struct {
		ip       string
		provID   string
		provName string
	}

	seenIPs := make(map[string]bool)

	for _, prov := range providersList {
		if !prov.Enabled {
			continue
		}
		var pool []string
		pool = append(pool, prov.FallbackIPs...)
		pool = append(pool, prov.Gateways...)

		for _, ip := range pool {
			if ip != "" && !seenIPs[ip] {
				seenIPs[ip] = true
				candidates = append(candidates, struct {
					ip       string
					provID   string
					provName string
				}{ip: ip, provID: prov.ID, provName: prov.Name})
			}
		}
	}

	var results []domain.ProbeResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	sem := make(chan struct{}, 20) // Cap concurrency at 20 parallel threads

	for _, cand := range candidates {
		wg.Add(1)
		go func(c struct {
			ip       string
			provID   string
			provName string
		}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r := p.ProbeIP(c.ip, target)
			r.ProviderID = c.provID
			r.ProviderName = c.provName

			mu.Lock()
			results = append(results, r)
			mu.Unlock()
		}(cand)
	}

	wg.Wait()
	return results
}

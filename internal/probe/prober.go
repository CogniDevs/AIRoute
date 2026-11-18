package probe

import (
	"crypto/tls"
	"fmt"
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
	case domain.TargetCursor:
		res.IsClean, res.Reason = p.checkCursor(ip)
	case domain.TargetAll, "":
		var passed []string
		if ok, _ := p.checkGemini(ip); ok {
			passed = append(passed, "Gemini")
		}
		if ok, _ := p.checkClaude(ip); ok {
			passed = append(passed, "Claude")
		}
		if ok, _ := p.checkChatGPT(ip); ok {
			passed = append(passed, "ChatGPT")
		}
		if ok, _ := p.checkAntigravity(ip); ok {
			passed = append(passed, "Antigravity")
		}
		if ok, _ := p.checkCursor(ip); ok {
			passed = append(passed, "Cursor")
		}

		if len(passed) > 0 {
			res.IsClean = true
			res.Reason = fmt.Sprintf("Clean Gateway (%s OK)", strings.Join(passed, ", "))
		} else {
			res.IsClean = false
			res.Reason = "Failed checks for all AI targets"
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

	var respBuilder strings.Builder
	buf := make([]byte, 2048)
	for {
		n, err := tlsConn.Read(buf)
		if n > 0 {
			respBuilder.Write(buf[:n])
			str := strings.ToLower(respBuilder.String())
			if strings.Contains(str, "\r\n\r\n") || respBuilder.Len() >= 2048 {
				break
			}
		}
		if err != nil {
			break
		}
	}
	resp := strings.ToLower(respBuilder.String())

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

	var respBuilder strings.Builder
	buf := make([]byte, 2048)
	for {
		n, err := tlsConn.Read(buf)
		if n > 0 {
			respBuilder.Write(buf[:n])
			if strings.Contains(respBuilder.String(), "\r\n\r\n") || respBuilder.Len() >= 2048 {
				break
			}
		}
		if err != nil {
			break
		}
	}
	resp := strings.ToLower(respBuilder.String())

	if strings.Contains(resp, "direct") || strings.Contains(resp, "comss.one") {
		return false, "Provider Stub (Not a proxy)"
	}
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

	var respBuilder strings.Builder
	buf := make([]byte, 2048)
	for {
		n, err := tlsConn.Read(buf)
		if n > 0 {
			respBuilder.Write(buf[:n])
			if strings.Contains(respBuilder.String(), "\r\n\r\n") || respBuilder.Len() >= 2048 {
				break
			}
		}
		if err != nil {
			break
		}
	}
	resp := strings.ToLower(respBuilder.String())

	if strings.Contains(resp, "direct") || strings.Contains(resp, "comss.one") {
		return false, "Provider Stub (Not a proxy)"
	}
	if strings.Contains(resp, "unable to load site") || strings.Contains(resp, "vpn") || strings.Contains(resp, "not available in your country") {
		return false, "OpenAI Region/VPN Block"
	}
	if strings.Contains(resp, "chatgpt") || strings.Contains(resp, "oai-did") || strings.Contains(resp, "cf-ray") || strings.Contains(resp, "server: cloudflare") {
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

	var respBuilder strings.Builder
	buf := make([]byte, 2048)
	for {
		n, err := tlsConn.Read(buf)
		if n > 0 {
			respBuilder.Write(buf[:n])
			str := strings.ToLower(respBuilder.String())
			if strings.Contains(str, "401") || strings.Contains(str, "403") || respBuilder.Len() >= 2048 {
				break
			}
		}
		if err != nil {
			break
		}
	}
	resp := strings.ToLower(respBuilder.String())

	if (strings.Contains(resp, "401") || strings.Contains(resp, "403")) &&
		(strings.Contains(resp, "cloudcode") || strings.Contains(resp, "google") || strings.Contains(resp, "unauthenticated") || strings.Contains(resp, "loadcodeassist")) {
		return true, "Clean Antigravity CloudCode Gateway"
	}

	return false, "CloudCode endpoint rejected or filtered"
}

func (p *Prober) checkCursor(ip string) (bool, string) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "443"), p.Timeout)
	if err != nil {
		return false, "TCP Timeout"
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tls.Config{ServerName: "cursor.com"})
	_ = tlsConn.SetDeadline(time.Now().Add(p.Timeout))
	if err := tlsConn.Handshake(); err != nil {
		return false, "TLS Invalid (Not Cursor cert)"
	}
	defer tlsConn.Close()

	req := "GET / HTTP/1.1\r\nHost: cursor.com\r\nUser-Agent: Mozilla/5.0\r\nConnection: close\r\n\r\n"
	_, _ = tlsConn.Write([]byte(req))

	var respBuilder strings.Builder
	buf := make([]byte, 2048)
	for {
		n, err := tlsConn.Read(buf)
		if n > 0 {
			respBuilder.Write(buf[:n])
			if strings.Contains(respBuilder.String(), "\r\n\r\n") || respBuilder.Len() >= 2048 {
				break
			}
		}
		if err != nil {
			break
		}
	}
	resp := strings.ToLower(respBuilder.String())

	if strings.Contains(resp, "direct") || strings.Contains(resp, "comss.one") {
		return false, "Provider Stub (Not a proxy)"
	}
	if strings.Contains(resp, "not available in your region") || strings.Contains(resp, "app-unavailable-in-region") {
		return false, "Regional Block (Cursor)"
	}
	if strings.Contains(resp, "server: cloudflare") || strings.Contains(resp, "cf-ray") || strings.Contains(resp, "cursor") {
		return true, "Clean Cursor Gateway"
	}

	return false, "Non-Cursor Gateway"
}

type providerCandidate struct {
	ip       string
	provID   string
	provName string
}

// ProbeAllProviders concurrently benchmarks and tests all candidate IPs across registered providers.
func (p *Prober) ProbeAllProviders(providersList []domain.Provider, target domain.ServiceTarget) []domain.ProbeResult {
	var domainsToQuery []string
	switch target {
	case domain.TargetGemini:
		domainsToQuery = []string{"gemini.google.com"}
	case domain.TargetClaude:
		domainsToQuery = []string{"claude.ai"}
	case domain.TargetChatGPT:
		domainsToQuery = []string{"chatgpt.com"}
	case domain.TargetAntigravity:
		domainsToQuery = []string{"daily-cloudcode-pa.googleapis.com"}
	case domain.TargetCursor:
		domainsToQuery = []string{"cursor.com"}
	default:
		domainsToQuery = []string{
			"gemini.google.com",
			"claude.ai",
			"chatgpt.com",
			"daily-cloudcode-pa.googleapis.com",
			"cursor.com",
		}
	}

	var results []domain.ProbeResult
	var mu sync.Mutex

	// Map of provider -> list of candidate IPs
	type provResolution struct {
		prov   domain.Provider
		ips    []string
		dohErr error
	}

	resolutionChan := make(chan provResolution, len(providersList))
	var resWg sync.WaitGroup

	for _, prov := range providersList {
		if !prov.Enabled {
			continue
		}
		resWg.Add(1)
		go func(pr domain.Provider) {
			defer resWg.Done()
			var resolvedIPs []string
			var lastErr error

			for _, d := range domainsToQuery {
				ips, err := ResolveViaProvider(pr, d, p.Timeout)
				if err != nil {
					lastErr = err
				} else {
					resolvedIPs = append(resolvedIPs, ips...)
				}
			}

			resolutionChan <- provResolution{
				prov:   pr,
				ips:    resolvedIPs,
				dohErr: lastErr,
			}
		}(prov)
	}

	resWg.Wait()
	close(resolutionChan)

	var candidates []providerCandidate
	seenPerProvider := make(map[string]bool) // Key: provID + "|" + ip

	for res := range resolutionChan {
		pr := res.prov
		var pool []string
		pool = append(pool, res.ips...)
		pool = append(pool, pr.Gateways...) // ONLY Gateways, NEVER FallbackIPs!

		countForProv := 0
		for _, ip := range pool {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			key := pr.ID + "|" + ip
			if !seenPerProvider[key] {
				seenPerProvider[key] = true
				candidates = append(candidates, providerCandidate{
					ip:       ip,
					provID:   pr.ID,
					provName: pr.Name,
				})
				countForProv++
			}
		}

		// If no IPs were obtained from DoH or Gateways, report the failure directly
		if countForProv == 0 {
			reason := "Нет доступных шлюзов"
			if res.dohErr != nil {
				reason = fmt.Sprintf("DoH ошибка: %v", res.dohErr)
			}
			results = append(results, domain.ProbeResult{
				ProviderID:   pr.ID,
				ProviderName: pr.Name,
				IP:           "",
				LatencyMs:    9999,
				IsClean:      false,
				Reason:       reason,
				Target:       target,
			})
		}
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 20) // Cap concurrency at 20 parallel threads

	for _, cand := range candidates {
		wg.Add(1)
		go func(c providerCandidate) {
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

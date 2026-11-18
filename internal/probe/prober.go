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

// ProbeTargetIP executes deep validation of an IP for a single concrete AI target,
// precisely measuring its own round-trip time.
func (p *Prober) ProbeTargetIP(ip string, target domain.ServiceTarget) domain.ProbeResult {
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
	default:
		res.IsClean, res.Reason = p.checkGemini(ip)
	}

	res.LatencyMs = time.Since(start).Milliseconds()
	return res
}

// ProbeIPTargets executes parallel checks of a single IP for multiple targets.
// Returns one ProbeResult per target with individual latency.
func (p *Prober) ProbeIPTargets(ip string, targets []domain.ServiceTarget) []domain.ProbeResult {
	if len(targets) == 0 {
		targets = domain.AllServiceTargets
	}

	results := make([]domain.ProbeResult, len(targets))
	var wg sync.WaitGroup

	for i, t := range targets {
		wg.Add(1)
		go func(idx int, target domain.ServiceTarget) {
			defer wg.Done()
			results[idx] = p.ProbeTargetIP(ip, target)
		}(i, t)
	}

	wg.Wait()
	return results
}

// ProbeIP validates an IP for the given target. If target is TargetAll or empty,
// all targets are checked concurrently and a summary ProbeResult is returned.
func (p *Prober) ProbeIP(ip string, target domain.ServiceTarget) domain.ProbeResult {
	if target != "" && target != domain.TargetAll {
		return p.ProbeTargetIP(ip, target)
	}

	results := p.ProbeIPTargets(ip, domain.AllServiceTargets)
	var passed []string
	var minCleanLatency int64 = 999999
	var bestTarget domain.ServiceTarget = domain.TargetGemini

	for _, r := range results {
		if r.IsClean {
			passed = append(passed, string(r.Target))
			if r.LatencyMs < minCleanLatency {
				minCleanLatency = r.LatencyMs
				bestTarget = r.Target
			}
		}
	}

	summary := domain.ProbeResult{
		IP:        ip,
		Target:    bestTarget,
		LatencyMs: minCleanLatency,
	}

	if len(passed) > 0 {
		summary.IsClean = true
		summary.Reason = fmt.Sprintf("Clean Gateway (%s OK)", strings.Join(passed, ", "))
	} else {
		summary.IsClean = false
		summary.LatencyMs = results[0].LatencyMs
		summary.Reason = "Failed checks for all AI targets"
	}

	return summary
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

func getDomainForTarget(target domain.ServiceTarget) string {
	switch target {
	case domain.TargetGemini:
		return "gemini.google.com"
	case domain.TargetClaude:
		return "claude.ai"
	case domain.TargetChatGPT:
		return "chatgpt.com"
	case domain.TargetAntigravity:
		return "daily-cloudcode-pa.googleapis.com"
	case domain.TargetCursor:
		return "cursor.com"
	default:
		return "gemini.google.com"
	}
}

// ProbeAllProvidersStream probes each provider concurrently and invokes onProviderDone
// as soon as each provider finishes benchmarking. Limits global network probe concurrency.
func (p *Prober) ProbeAllProvidersStream(
	providersList []domain.Provider,
	targets []domain.ServiceTarget,
	onProviderStart func(provID string),
	onProviderDone func(provID string, results []domain.ProbeResult),
) []domain.ProbeResult {
	if len(targets) == 0 {
		targets = domain.AllServiceTargets
	}

	var allResults []domain.ProbeResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Global concurrency limiter for probe socket connections
	probeSem := make(chan struct{}, 25)

	for _, prov := range providersList {
		if !prov.Enabled {
			continue
		}

		if onProviderStart != nil {
			onProviderStart(prov.ID)
		}

		wg.Add(1)
		go func(pr domain.Provider) {
			defer wg.Done()

			var provResults []domain.ProbeResult

			// 1. Resolve domain for each target via DoH / DNS
			resolvedMap := make(map[domain.ServiceTarget][]string)
			var lastErr error

			for _, t := range targets {
				testDomain := getDomainForTarget(t)
				ips, err := ResolveViaProvider(pr, testDomain, p.Timeout)
				if err != nil {
					lastErr = err
				} else {
					resolvedMap[t] = ips
				}
			}

			// 2. Candidate IPs per provider: deduplicated pool of resolved + Gateways (excluding fallback IPs)
			poolSet := make(map[string]bool)
			for _, ips := range resolvedMap {
				for _, ip := range ips {
					ip = strings.TrimSpace(ip)
					if ip != "" {
						poolSet[ip] = true
					}
				}
			}
			for _, gw := range pr.Gateways {
				gw = strings.TrimSpace(gw)
				if gw != "" {
					parsed := net.ParseIP(gw)
					if parsed != nil && !IsFilteredGatewayIP(parsed) {
						poolSet[gw] = true
					}
				}
			}

			// If no candidates found, emit failure results for targets
			if len(poolSet) == 0 {
				reason := "Нет доступных шлюзов"
				if lastErr != nil {
					reason = fmt.Sprintf("DoH ошибка: %v", lastErr)
				}
				for _, t := range targets {
					r := domain.ProbeResult{
						ProviderID:   pr.ID,
						ProviderName: pr.Name,
						IP:           "",
						LatencyMs:    9999,
						IsClean:      false,
						Reason:       reason,
						Target:       t,
					}
					provResults = append(provResults, r)
				}
			} else {
				// Probe each candidate IP against relevant targets
				var pWg sync.WaitGroup
				var pMu sync.Mutex

				for ip := range poolSet {
					for _, t := range targets {
						pWg.Add(1)
						go func(candIP string, target domain.ServiceTarget) {
							defer pWg.Done()
							probeSem <- struct{}{}
							defer func() { <-probeSem }()

							r := p.ProbeTargetIP(candIP, target)
							r.ProviderID = pr.ID
							r.ProviderName = pr.Name

							pMu.Lock()
							provResults = append(provResults, r)
							pMu.Unlock()
						}(ip, t)
					}
				}
				pWg.Wait()
			}

			if onProviderDone != nil {
				onProviderDone(pr.ID, provResults)
			}

			mu.Lock()
			allResults = append(allResults, provResults...)
			mu.Unlock()
		}(prov)
	}

	wg.Wait()
	return allResults
}

// ProbeAllProviders concurrently benchmarks all providers for a target (backward compatibility).
func (p *Prober) ProbeAllProviders(providersList []domain.Provider, target domain.ServiceTarget) []domain.ProbeResult {
	var targets []domain.ServiceTarget
	if target == "" || target == domain.TargetAll {
		targets = domain.AllServiceTargets
	} else {
		targets = []domain.ServiceTarget{target}
	}
	return p.ProbeAllProvidersStream(providersList, targets, nil, nil)
}

// VerifyActiveRoutes quickly tests only the active IPs configured in routes.
func (p *Prober) VerifyActiveRoutes(routes map[domain.ServiceTarget]domain.ServiceRoute) []domain.ProbeResult {
	var results []domain.ProbeResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	for t, r := range routes {
		if !r.Enabled || strings.TrimSpace(r.IP) == "" {
			continue
		}
		wg.Add(1)
		go func(target domain.ServiceTarget, route domain.ServiceRoute) {
			defer wg.Done()
			res := p.ProbeTargetIP(route.IP, target)
			res.ProviderID = route.ProviderID
			res.ProviderName = route.ProviderName

			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}(t, r)
	}

	wg.Wait()
	return results
}

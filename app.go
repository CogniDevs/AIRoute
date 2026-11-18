package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"airoute/internal/domain"
	"airoute/internal/hosts"
	"airoute/internal/probe"
	"airoute/internal/providers"
)

// App struct manages core backend services and bridges with Wails frontend.
type App struct {
	ctx          context.Context
	providersMgr *providers.Manager
	hostsMgr     *hosts.Manager
	prober       *probe.Prober

	mu            sync.RWMutex
	lastLatencyMs int64
	lastProbes    map[string][]domain.ProbeResult // Key: target
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		providersMgr: providers.NewManager(),
		hostsMgr:     hosts.NewManager(),
		prober:       probe.NewProber(3 * time.Second),
		lastProbes:   make(map[string][]domain.ProbeResult),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetStatus returns the current route state and whether hosts are currently active.
func (a *App) GetStatus() domain.RouteState {
	isActive, ip, target, provider := a.hostsMgr.GetActiveState()

	a.mu.RLock()
	lat := a.lastLatencyMs
	a.mu.RUnlock()

	state := domain.RouteState{
		IsEnabled:      isActive,
		ActiveMode:     domain.ModeHosts,
		ActiveTarget:   target,
		ActiveProvider: provider,
		ActiveIP:       ip,
		LatencyMs:      lat,
		LastUpdated:    time.Now(),
	}

	if isActive {
		state.Message = fmt.Sprintf("Маршрутизация активна (%s через %s)", ip, provider)
	} else {
		state.Message = "Маршрутизация отключена (дефолтный DNS)"
	}

	return state
}

// ToggleRoute enables or disables smart routing for the requested target.
func (a *App) ToggleRoute(enable bool, target domain.ServiceTarget, providerID string) (domain.RouteState, error) {
	if !enable {
		if err := a.hostsMgr.ClearRoute(); err != nil {
			return a.GetStatus(), fmt.Errorf("не удалось отключить маршрутизацию: %w", err)
		}
		a.mu.Lock()
		a.lastLatencyMs = 0
		a.mu.Unlock()
		return a.GetStatus(), nil
	}

	if target == "" {
		target = domain.TargetAll
	}

	// 1. Find chosen provider
	var chosenProvider domain.Provider
	if providerID != "" {
		if p, found := a.providersMgr.GetByID(providerID); found {
			chosenProvider = p
		}
	}

	if chosenProvider.ID == "" {
		all := a.providersMgr.GetAll()
		if len(all) == 0 {
			return a.GetStatus(), fmt.Errorf("список DNS провайдеров пуст")
		}
		// Prefer dns-ai-cf if available, else first
		for _, p := range all {
			if p.ID == "dns-ai-cf" {
				chosenProvider = p
				break
			}
		}
		if chosenProvider.ID == "" {
			chosenProvider = all[0]
		}
	}

	// 2. Determine target services
	var services []domain.ServiceTarget
	if target == domain.TargetAll {
		services = []domain.ServiceTarget{
			domain.TargetGemini,
			domain.TargetClaude,
			domain.TargetChatGPT,
			domain.TargetAntigravity,
			domain.TargetCursor,
		}
	} else {
		services = []domain.ServiceTarget{target}
	}

	serviceIPs := make(map[domain.ServiceTarget]string)
	serviceLatencies := make(map[domain.ServiceTarget]int64)
	var lastErrReason string

	// 3. For each service, resolve via DoH/DNS and probe candidate gateways
	for _, srv := range services {
		var testDomain string
		switch srv {
		case domain.TargetGemini:
			testDomain = "gemini.google.com"
		case domain.TargetClaude:
			testDomain = "claude.ai"
		case domain.TargetChatGPT:
			testDomain = "chatgpt.com"
		case domain.TargetAntigravity:
			testDomain = "daily-cloudcode-pa.googleapis.com"
		case domain.TargetCursor:
			testDomain = "cursor.com"
		default:
			testDomain = "gemini.google.com"
		}

		resolvedIPs, err := probe.ResolveViaProvider(chosenProvider, testDomain, a.prober.Timeout)
		if err != nil {
			lastErrReason = fmt.Sprintf("DoH резолв %s: %v", testDomain, err)
		}

		// Candidates = resolved IPs + provider gateways (NEVER FallbackIPs)
		var candidates []string
		candidates = append(candidates, resolvedIPs...)
		candidates = append(candidates, chosenProvider.Gateways...)

		seen := make(map[string]bool)
		var bestIP string
		var bestLat int64 = 999999

		for _, ip := range candidates {
			ip = strings.TrimSpace(ip)
			if ip == "" || seen[ip] {
				continue
			}
			seen[ip] = true

			res := a.prober.ProbeIP(ip, srv)
			if res.IsClean {
				if bestIP == "" || res.LatencyMs < bestLat {
					bestIP = ip
					bestLat = res.LatencyMs
				}
			} else {
				if lastErrReason == "" {
					lastErrReason = res.Reason
				}
			}
		}

		if bestIP != "" {
			serviceIPs[srv] = bestIP
			serviceLatencies[srv] = bestLat
		}
	}

	// 4. Validate that we found at least one clean gateway
	if target != domain.TargetAll && serviceIPs[target] == "" {
		if lastErrReason == "" {
			lastErrReason = "шлюзы недоступны или заблокированы"
		}
		return a.GetStatus(), fmt.Errorf("не найден рабочий шлюз для %s через %s (%s)", target, chosenProvider.Name, lastErrReason)
	}

	if target == domain.TargetAll && len(serviceIPs) == 0 {
		if lastErrReason == "" {
			lastErrReason = "шлюзы недоступны или заблокированы"
		}
		return a.GetStatus(), fmt.Errorf("не найден ни один рабочий шлюз через %s (%s)", chosenProvider.Name, lastErrReason)
	}

	// 5. Select overall best IP and latency
	var bestOverallIP string
	var bestOverallLat int64 = 999999

	for srv, ip := range serviceIPs {
		lat := serviceLatencies[srv]
		if bestOverallIP == "" || lat < bestOverallLat {
			bestOverallIP = ip
			bestOverallLat = lat
		}
	}

	// 6. Apply to hosts
	if err := a.hostsMgr.ApplyMultiRoute(target, serviceIPs, bestOverallIP, chosenProvider.Name); err != nil {
		return a.GetStatus(), fmt.Errorf("ошибка записи hosts: %w", err)
	}

	a.mu.Lock()
	a.lastLatencyMs = bestOverallLat
	a.mu.Unlock()

	status := a.GetStatus()
	status.ActiveIP = bestOverallIP
	status.LatencyMs = bestOverallLat
	return status, nil
}

// TestProviders scans and benchmarks all registered providers for the given AI target.
func (a *App) TestProviders(target domain.ServiceTarget) []domain.ProbeResult {
	if target == "" {
		target = domain.TargetAll
	}
	all := a.providersMgr.GetAll()
	results := a.prober.ProbeAllProviders(all, target)

	a.mu.Lock()
	a.lastProbes[string(target)] = results
	a.mu.Unlock()

	return results
}

// GetProviders returns the registered DNS / DoH catalog.
func (a *App) GetProviders() []domain.Provider {
	return a.providersMgr.GetAll()
}

// AddCustomProvider adds or updates a user-defined provider.
func (a *App) AddCustomProvider(p domain.Provider) error {
	if p.ID == "" {
		p.ID = fmt.Sprintf("custom-%d", time.Now().Unix())
	}
	return a.providersMgr.Add(p)
}

// RemoveCustomProvider removes a custom provider by ID.
func (a *App) RemoveCustomProvider(id string) error {
	return a.providersMgr.Remove(id)
}

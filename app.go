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
	"airoute/internal/proxy"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct manages core backend services and bridges with Wails frontend.
type App struct {
	ctx          context.Context
	providersMgr *providers.Manager
	hostsMgr     *hosts.Manager
	prober       *probe.Prober
	proxyServer  *proxy.Server

	mu           sync.RWMutex
	latestProbes []domain.ProbeResult
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		providersMgr: providers.NewManager(),
		hostsMgr:     hosts.NewManager(),
		prober:       probe.NewProber(3 * time.Second),
		proxyServer:  proxy.NewServer(),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetStatus returns the current route state, parsing hosts and proxy status.
func (a *App) GetStatus() domain.RouteState {
	isActive, routes, patchedCount, totalCount := a.hostsMgr.GetActiveRoutes()

	proxyRunning := a.proxyServer.IsRunning()
	proxyAddr := a.proxyServer.GetAddr()

	mode := domain.ModeHosts
	if proxyRunning && !isActive {
		mode = domain.ModeProxy
	}

	state := domain.RouteState{
		IsEnabled:    isActive || proxyRunning,
		ActiveMode:   mode,
		LastUpdated:  time.Now(),
		Routes:       routes,
		PatchedCount: patchedCount,
		TotalCount:   totalCount,
		HostsPath:    a.hostsMgr.GetHostsPath(),
		ProxyRunning: proxyRunning,
		ProxyAddr:    proxyAddr,
	}

	if isActive {
		state.Message = fmt.Sprintf("Hosts пропатчен: активно %d из %d сервисов", patchedCount, totalCount)
	} else if proxyRunning {
		state.Message = fmt.Sprintf("Локальный прокси активен (%s)", proxyAddr)
	} else {
		state.Message = "Маршрутизация выключена (дефолтный DNS системы)"
	}

	return state
}

// GetHostsPath returns the detected system hosts file path.
func (a *App) GetHostsPath() string {
	return a.hostsMgr.GetHostsPath()
}

// TestProviders streams benchmarks across all providers and AI targets.
func (a *App) TestProviders(target domain.ServiceTarget) []domain.ProbeResult {
	targets := domain.AllServiceTargets
	if target != "" && target != domain.TargetAll {
		targets = []domain.ServiceTarget{target}
	}

	allProviders := a.providersMgr.GetAll()
	var provIDs []string
	for _, p := range allProviders {
		if p.Enabled {
			provIDs = append(provIDs, p.ID)
		}
	}

	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "probe:start", provIDs)
	}

	onStart := func(provID string) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "probe:provider_start", provID)
		}
	}

	onDone := func(provID string, results []domain.ProbeResult) {
		a.mu.Lock()
		// Remove previous results for this provider
		var filtered []domain.ProbeResult
		for _, r := range a.latestProbes {
			if r.ProviderID != provID {
				filtered = append(filtered, r)
			}
		}
		a.latestProbes = append(filtered, results...)
		a.mu.Unlock()

		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "probe:provider", map[string]interface{}{
				"provider_id": provID,
				"results":     results,
			})
		}
	}

	results := a.prober.ProbeAllProvidersStream(allProviders, targets, onStart, onDone)

	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "probe:done", map[string]interface{}{
			"results": results,
		})
	}

	return results
}

// ApplyAutoBest automatically resolves the lowest latency clean IP for each enabled target and patches hosts.
func (a *App) ApplyAutoBest(enabledTargets []domain.ServiceTarget) (domain.RouteState, error) {
	if len(enabledTargets) == 0 {
		enabledTargets = domain.AllServiceTargets
	}

	selections := make(map[domain.ServiceTarget]string)
	for _, t := range enabledTargets {
		selections[t] = "auto"
	}

	return a.ApplyRoutes(selections, enabledTargets)
}

// ApplyRoutes applies selected gateways per target to hosts and local proxy.
func (a *App) ApplyRoutes(selections map[domain.ServiceTarget]string, enabledTargets []domain.ServiceTarget) (domain.RouteState, error) {
	enabledMap := make(map[domain.ServiceTarget]bool)
	for _, t := range enabledTargets {
		enabledMap[t] = true
	}

	routes := make(map[domain.ServiceTarget]domain.ServiceRoute)

	// Check if we need to run initial scan
	a.mu.RLock()
	probesCount := len(a.latestProbes)
	a.mu.RUnlock()

	if probesCount == 0 {
		_ = a.TestProviders(domain.TargetAll)
	}

	for _, target := range domain.AllServiceTargets {
		if !enabledMap[target] {
			routes[target] = domain.ServiceRoute{
				Target:  target,
				Enabled: false,
			}
			continue
		}

		provID := selections[target]
		if provID == "" {
			provID = "auto"
		}

		bestIP, provName, resolvedProvID, lat := a.findBestGateway(target, provID)
		if bestIP == "" {
			// Try on-demand scan if not found
			bestIP, provName, resolvedProvID, lat = a.probeOnDemand(target, provID)
		}

		if bestIP == "" {
			return a.GetStatus(), fmt.Errorf("не удалось найти рабочий чистый шлюз для сервиса %s", target)
		}

		routes[target] = domain.ServiceRoute{
			Target:       target,
			Enabled:      true,
			ProviderID:   resolvedProvID,
			ProviderName: provName,
			IP:           bestIP,
			LatencyMs:    lat,
		}
	}

	// Apply to system hosts file
	if err := a.hostsMgr.ApplyServiceRoutes(routes); err != nil {
		return a.GetStatus(), fmt.Errorf("ошибка записи в hosts: %w", err)
	}

	// If local proxy is running, synchronize its routes
	if a.proxyServer.IsRunning() {
		a.proxyServer.UpdateRoutes(routes)
	}

	return a.GetStatus(), nil
}

// ClearRoutes removes the AIRoute section from the hosts file.
func (a *App) ClearRoutes() (domain.RouteState, error) {
	if err := a.hostsMgr.ClearRoute(); err != nil {
		return a.GetStatus(), fmt.Errorf("ошибка очистки hosts: %w", err)
	}
	return a.GetStatus(), nil
}

// VerifyActive performs a quick concurrent probe of only currently routed active IPs and updates routes.
func (a *App) VerifyActive() domain.RouteState {
	isActive, routes, _, _ := a.hostsMgr.GetActiveRoutes()
	if !isActive {
		return a.GetStatus()
	}

	results := a.prober.VerifyActiveRoutes(routes)

	a.mu.Lock()
	for _, r := range results {
		found := false
		for i, p := range a.latestProbes {
			if p.Target == r.Target && p.ProviderID == r.ProviderID && p.IP == r.IP {
				a.latestProbes[i] = r
				found = true
				break
			}
		}
		if !found {
			a.latestProbes = append(a.latestProbes, r)
		}

		if route, exists := routes[r.Target]; exists {
			route.LatencyMs = r.LatencyMs
			routes[r.Target] = route
		}
	}
	a.mu.Unlock()

	// Persist fresh latencies in hosts metadata comments
	_ = a.hostsMgr.ApplyServiceRoutes(routes)

	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "probe:done", map[string]interface{}{
			"results": results,
		})
	}

	return a.GetStatus()
}

// UpdateProvider adds or updates a provider (custom or overridden built-in).
func (a *App) UpdateProvider(p domain.Provider) error {
	return a.providersMgr.Update(p)
}

// ToggleProvider enables or disables a provider.
func (a *App) ToggleProvider(id string, enabled bool) error {
	return a.providersMgr.Toggle(id, enabled)
}

// ResetDefaultProviders resets all providers back to factory defaults.
func (a *App) ResetDefaultProviders() error {
	return a.providersMgr.Reset()
}

// SyncCatalog pulls latest online smart DNS servers and updates the local registry.
func (a *App) SyncCatalog() (int, error) {
	return a.providersMgr.SyncOnlineCatalog()
}

// ToggleProxy enables or disables embedded local HTTP CONNECT proxy.
func (a *App) ToggleProxy(enable bool, port int) (domain.RouteState, error) {
	if port <= 0 {
		port = 2080
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)

	if !enable {
		if err := a.proxyServer.Stop(); err != nil {
			return a.GetStatus(), fmt.Errorf("ошибка остановки прокси: %w", err)
		}
		return a.GetStatus(), nil
	}

	_, routes, _, _ := a.hostsMgr.GetActiveRoutes()
	a.proxyServer.UpdateRoutes(routes)

	if err := a.proxyServer.Start(addr); err != nil {
		return a.GetStatus(), fmt.Errorf("ошибка запуска прокси на %s: %w", addr, err)
	}

	return a.GetStatus(), nil
}

// ToggleRoute provides backward compatibility for single-switch toggling.
func (a *App) ToggleRoute(enable bool, target domain.ServiceTarget, providerID string) (domain.RouteState, error) {
	if !enable {
		return a.ClearRoutes()
	}
	targets := domain.AllServiceTargets
	if target != "" && target != domain.TargetAll {
		targets = []domain.ServiceTarget{target}
	}
	selections := make(map[domain.ServiceTarget]string)
	for _, t := range targets {
		selections[t] = providerID
	}
	return a.ApplyRoutes(selections, targets)
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

func (a *App) findBestGateway(target domain.ServiceTarget, provID string) (bestIP string, provName string, resolvedProvID string, minLat int64) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	minLat = 999999
	for _, r := range a.latestProbes {
		if r.Target != target || !r.IsClean || strings.TrimSpace(r.IP) == "" {
			continue
		}

		if provID != "auto" && provID != "" && r.ProviderID != provID {
			continue
		}

		if r.LatencyMs < minLat {
			minLat = r.LatencyMs
			bestIP = r.IP
			provName = r.ProviderName
			resolvedProvID = r.ProviderID
		}
	}

	if resolvedProvID == "" && provID != "auto" {
		resolvedProvID = provID
		if p, found := a.providersMgr.GetByID(provID); found {
			provName = p.Name
		}
	}

	return bestIP, provName, resolvedProvID, minLat
}

func (a *App) probeOnDemand(target domain.ServiceTarget, provID string) (string, string, string, int64) {
	var providersList []domain.Provider
	if provID == "auto" || provID == "" {
		providersList = a.providersMgr.GetAll()
	} else if p, found := a.providersMgr.GetByID(provID); found {
		providersList = []domain.Provider{p}
	} else {
		return "", "", provID, 0
	}

	results := a.prober.ProbeAllProvidersStream(providersList, []domain.ServiceTarget{target}, nil, nil)
	a.mu.Lock()
	a.latestProbes = append(a.latestProbes, results...)
	a.mu.Unlock()

	var bestIP, provName, resolvedProvID string
	var minLat int64 = 999999

	for _, r := range results {
		if r.Target == target && r.IsClean && strings.TrimSpace(r.IP) != "" {
			if r.LatencyMs < minLat {
				minLat = r.LatencyMs
				bestIP = r.IP
				provName = r.ProviderName
				resolvedProvID = r.ProviderID
			}
		}
	}

	return bestIP, provName, resolvedProvID, minLat
}

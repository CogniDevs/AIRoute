package main

import (
	"context"
	"fmt"
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
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		providersMgr: providers.NewManager(),
		hostsMgr:     hosts.NewManager(),
		prober:       probe.NewProber(3 * time.Second),
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetStatus returns the current route state and whether hosts are currently active.
func (a *App) GetStatus() domain.RouteState {
	isActive, ip, target, provider := a.hostsMgr.GetActiveState()
	state := domain.RouteState{
		IsEnabled:      isActive,
		ActiveMode:     domain.ModeHosts,
		ActiveTarget:   target,
		ActiveProvider: provider,
		ActiveIP:       ip,
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
		return a.GetStatus(), nil
	}

	// If enabling, find the best provider or specified provider
	var chosenProvider domain.Provider
	if providerID != "" {
		if p, found := a.providersMgr.GetByID(providerID); found {
			chosenProvider = p
		}
	}

	if chosenProvider.ID == "" {
		// Select default highest priority (e.g. DNS-AI)
		all := a.providersMgr.GetAll()
		if len(all) > 0 {
			chosenProvider = all[0]
		} else {
			return a.GetStatus(), fmt.Errorf("список DNS провайдеров пуст")
		}
	}

	// Pick best candidate IP
	var candidateIP string
	if len(chosenProvider.FallbackIPs) > 0 {
		candidateIP = chosenProvider.FallbackIPs[0]
	} else if len(chosenProvider.Gateways) > 0 {
		candidateIP = chosenProvider.Gateways[0]
	} else {
		return a.GetStatus(), fmt.Errorf("у провайдера %s нет доступных IP", chosenProvider.Name)
	}

	if target == "" {
		target = domain.TargetAll
	}

	if err := a.hostsMgr.ApplyRoute(target, candidateIP, chosenProvider.Name); err != nil {
		return a.GetStatus(), fmt.Errorf("ошибка записи hosts: %w", err)
	}

	return a.GetStatus(), nil
}

// TestProviders scans and benchmarks all registered providers for the given AI target.
func (a *App) TestProviders(target domain.ServiceTarget) []domain.ProbeResult {
	if target == "" {
		target = domain.TargetAll
	}
	all := a.providersMgr.GetAll()
	return a.prober.ProbeAllProviders(all, target)
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

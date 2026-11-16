package providers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"airoute/internal/domain"
)

var defaultProviders = []domain.Provider{
	{
		ID:          "dns-ai",
		Name:        "DNS-AI.RU (Шифрованный Smart DNS)",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.dns-ai.ru/dns-query",
		FallbackIPs: []string{"194.87.196.13", "45.151.102.219"},
		Gateways:    []string{"194.87.196.13", "45.151.102.219"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "geohide-eu",
		Name:        "GeoHide Europe",
		Kind:        domain.KindDoH,
		Endpoint:    "https://eu.geohide.ru/dns-query",
		FallbackIPs: []string{"31.25.239.132", "5.175.161.3", "37.221.93.141"},
		Gateways:    []string{"31.25.239.132", "5.175.161.40", "5.175.161.3", "45.13.237.213"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "bezmezhau",
		Name:        "Bezmezhau Smart DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.bezmezhau.com/dns-query",
		FallbackIPs: []string{"143.20.64.55", "143.20.64.192"},
		Gateways:    []string{"143.20.64.55", "143.20.64.192", "31.58.216.201"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "magicdns",
		Name:        "MagicDNS Europe Pool",
		Kind:        domain.KindDoH,
		Endpoint:    "https://api.magicdns.ru/sync",
		FallbackIPs: []string{"109.120.132.43", "94.249.177.102"},
		Gateways: []string{
			"45.83.27.4", "45.83.27.27", "45.83.27.45", "45.83.27.79",
			"45.83.27.120", "45.83.27.171", "45.83.27.213", "45.83.27.241",
			"109.120.132.43", "94.249.177.102", "109.120.177.170", "45.86.65.181",
		},
		Enabled:  true,
		IsCustom: false,
	},
	{
		ID:          "zapret",
		Name:        "Zapret DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.zapret.moe/dns-query",
		FallbackIPs: []string{"172.67.164.60"},
		Gateways:    []string{"5.129.221.64", "5.129.221.79", "172.67.164.60"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "quicbest",
		Name:        "QuicBest DoH",
		Kind:        domain.KindDoH,
		Endpoint:    "https://w.quic.best/dns-query",
		FallbackIPs: []string{"194.238.57.69"},
		Gateways:    []string{"194.238.57.69"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "wlrus",
		Name:        "WLRUS Smart DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.wlrus.cfd/dns-query",
		FallbackIPs: []string{"138.124.254.134", "95.85.240.254"},
		Gateways:    []string{"191.217.212.16", "138.124.254.134", "95.85.240.254"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "malw",
		Name:        "Malw.link Gateway",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.malw.link/dns-query",
		FallbackIPs: []string{"179.254.166.171", "164.5.224.81"},
		Gateways:    []string{"179.254.166.174", "179.254.166.171", "164.5.224.81", "185.31.40.18"},
		Enabled:     true,
		IsCustom:    false,
	},
}

// Manager handles in-memory and persisted user providers.
type Manager struct {
	mu        sync.RWMutex
	providers []domain.Provider
	cfgPath   string
}

// NewManager creates a provider manager, restoring custom additions from disk.
func NewManager() *Manager {
	m := &Manager{
		providers: append([]domain.Provider(nil), defaultProviders...),
		cfgPath:   getConfigFilePath(),
	}
	m.loadCustom()
	return m
}

// GetAll returns a copy of all registered providers.
func (m *Manager) GetAll() []domain.Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res := make([]domain.Provider, len(m.providers))
	copy(res, m.providers)
	return res
}

// GetByID finds a provider by ID.
func (m *Manager) GetByID(id string) (domain.Provider, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, p := range m.providers {
		if p.ID == id {
			return p, true
		}
	}
	return domain.Provider{}, false
}

// Add adds or updates a custom provider.
func (m *Manager) Add(p domain.Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p.IsCustom = true
	found := false
	for i, exist := range m.providers {
		if exist.ID == p.ID {
			m.providers[i] = p
			found = true
			break
		}
	}
	if !found {
		m.providers = append(m.providers, p)
	}

	return m.saveCustomLocked()
}

// Remove deletes a custom provider by ID.
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.providers {
		if p.ID == id && p.IsCustom {
			m.providers = append(m.providers[:i], m.providers[i+1:]...)
			return m.saveCustomLocked()
		}
	}
	return nil
}

func getConfigFilePath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	targetDir := filepath.Join(configDir, "AIRoute")
	_ = os.MkdirAll(targetDir, 0755)
	return filepath.Join(targetDir, "custom_providers.json")
}

func (m *Manager) loadCustom() {
	data, err := os.ReadFile(m.cfgPath)
	if err != nil {
		return
	}
	var custom []domain.Provider
	if err := json.Unmarshal(data, &custom); err == nil {
		for _, cp := range custom {
			cp.IsCustom = true
			m.providers = append(m.providers, cp)
		}
	}
}

func (m *Manager) saveCustomLocked() error {
	var custom []domain.Provider
	for _, p := range m.providers {
		if p.IsCustom {
			custom = append(custom, p)
		}
	}
	data, err := json.MarshalIndent(custom, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfgPath, data, 0644)
}

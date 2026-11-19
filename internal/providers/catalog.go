package providers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"airoute/internal/domain"
)

var defaultProviders = []domain.Provider{
	{
		ID:          "geohide-eu",
		Name:        "GeoHide Europe",
		Kind:        domain.KindDoH,
		Endpoint:    "https://eu.geohide.ru/dns-query",
		FallbackIPs: []string{"5.175.161.3"},
		Gateways:    []string{"193.233.112.67", "193.233.112.68", "31.25.239.132", "5.175.161.40", "45.13.237.213", "37.221.93.141", "37.221.93.128"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "zapret",
		Name:        "Zapret DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.zapret.moe/dns-query",
		FallbackIPs: []string{"172.67.164.60"},
		Gateways:    []string{"83.217.211.149", "5.129.221.64", "5.129.221.79"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "geodema",
		Name:        "Geodema DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.geodema.love/dns-query",
		FallbackIPs: []string{"179.236.130.10", "179.236.130.11"},
		Gateways:    []string{"179.236.130.10", "179.236.130.11"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "semqka",
		Name:        "Semqka DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.semqka.xyz/dns-query",
		FallbackIPs: []string{"31.77.59.46"},
		Gateways:    []string{"31.77.59.46"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "dagami",
		Name:        "Dagami DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.dagami.ru/dns-query",
		FallbackIPs: []string{"185.149.145.6"},
		Gateways:    []string{"185.149.145.6"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "webshift",
		Name:        "WebShift DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://d.webshiftvpn.ru/dns-query",
		FallbackIPs: []string{"31.31.207.68"},
		Gateways:    []string{"31.31.207.68"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "magicdns",
		Name:        "MagicDNS Europe Pool",
		Kind:        domain.KindDoH,
		Endpoint:    "https://api.magicdns.ru/sync",
		FallbackIPs: []string{"109.120.177.170", "45.88.163.92"},
		Gateways: []string{
			"45.83.27.4", "45.83.27.27", "45.83.27.45", "45.83.27.55", "45.83.27.65",
			"45.83.27.72", "45.83.27.79", "45.83.27.81", "45.83.27.83", "45.83.27.91",
			"45.83.27.94", "45.83.27.99", "45.83.27.106", "45.83.27.120", "45.83.27.137",
			"45.83.27.156", "45.83.27.162", "45.83.27.168", "45.83.27.171", "45.83.27.179",
			"45.83.27.181", "45.83.27.195", "45.83.27.201", "45.83.27.202", "45.83.27.213",
			"45.83.27.218", "45.83.27.219", "45.83.27.228", "45.83.27.234", "45.83.27.241",
			"45.83.27.242",
		},
		Enabled:  true,
		IsCustom: false,
	},
	{
		ID:          "bezmezhau",
		Name:        "Bezmezhau Smart DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.bezmezhau.com/dns-query",
		FallbackIPs: []string{"143.20.64.55"},
		Gateways:    []string{"143.20.64.192", "31.58.216.201"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "quicbest",
		Name:        "w.quic.best DoH",
		Kind:        domain.KindDoH,
		Endpoint:    "https://w.quic.best/dns-query",
		FallbackIPs: []string{"201.58.191.60"},
		Gateways:    []string{"201.58.191.60"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "wlrus",
		Name:        "WLRUS Smart DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.wlrus.cfd/dns-query",
		FallbackIPs: []string{"95.85.240.254", "138.124.254.134"},
		Gateways:    []string{"191.217.212.16", "95.85.240.254", "138.124.254.134"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "dns-ai-cf",
		Name:        "DNS-AI.RU (Cloudflare CDN)",
		Kind:        domain.KindDoH,
		Endpoint:    "https://cf.dns-ai.ru/dns-query",
		FallbackIPs: nil,
		Gateways:    nil,
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "dns-ai",
		Name:        "DNS-AI.RU (Прямой Smart DNS)",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.dns-ai.ru/dns-query",
		FallbackIPs: []string{"192.144.59.14", "186.246.49.127"},
		Gateways:    []string{"192.144.59.14", "186.246.49.127"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "tamaev",
		Name:        "Tamaev DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://tamaev-gifts.xyz/dns-query",
		FallbackIPs: []string{"2.26.80.94"},
		Gateways:    []string{"2.26.80.94"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "comss",
		Name:        "Comss.one DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://dns.comss.one/dns-query",
		FallbackIPs: []string{"83.220.169.155"},
		Gateways:    []string{"83.220.169.155", "212.109.195.93", "195.133.25.16"},
		Enabled:     true,
		IsCustom:    false,
	},
	{
		ID:          "shecan",
		Name:        "Shecan DNS",
		Kind:        domain.KindDoH,
		Endpoint:    "https://free.shecan.ir/dns-query",
		FallbackIPs: []string{"185.51.200.2"},
		Gateways:    []string{"185.51.200.2", "178.22.122.100"},
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

// NewManager creates a provider manager, restoring custom additions and edits from disk.
func NewManager() *Manager {
	m := &Manager{
		providers: append([]domain.Provider(nil), defaultProviders...),
		cfgPath:   getConfigFilePath(),
	}
	m.loadPersisted()
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

// Add adds or updates a provider and persists changes.
func (m *Manager) Add(p domain.Provider) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	for i, exist := range m.providers {
		if exist.ID == p.ID {
			m.providers[i] = p
			found = true
			break
		}
	}
	if !found {
		p.IsCustom = true
		m.providers = append(m.providers, p)
	}

	return m.savePersistedLocked()
}

// Update edits existing provider properties or creates a new one.
func (m *Manager) Update(p domain.Provider) error {
	return m.Add(p)
}

// Toggle flips the enabled state of a provider.
func (m *Manager) Toggle(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.providers {
		if p.ID == id {
			m.providers[i].Enabled = enabled
			return m.savePersistedLocked()
		}
	}
	return fmt.Errorf("provider %s not found", id)
}

// Remove deletes a provider by ID (or disables if default).
func (m *Manager) Remove(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.providers {
		if p.ID == id {
			if p.IsCustom {
				m.providers = append(m.providers[:i], m.providers[i+1:]...)
			} else {
				m.providers[i].Enabled = false
			}
			return m.savePersistedLocked()
		}
	}
	return nil
}

// Reset restores the factory default providers list.
func (m *Manager) Reset() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers = append([]domain.Provider(nil), defaultProviders...)
	_ = os.Remove(m.cfgPath)
	return nil
}

// SyncOnlineCatalog fetches the latest bypass DNS directory from online registry.
func (m *Manager) SyncOnlineCatalog() (int, error) {
	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Get("https://dns.quic.best/data.js")
	if err != nil {
		return 0, fmt.Errorf("fetch online catalog: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("read catalog body: %w", err)
	}

	text := string(body)
	// Regex match bypass providers: name: '...', ... doh: '...'
	itemRegex := regexp.MustCompile(`(?s)name:\s*'([^']+)',\s*region:\s*'bypass'.*?doh:\s*'([^']+)'`)
	ipv4Regex := regexp.MustCompile(`ipv4:\s*\[([^\]]+)\]`)

	matches := itemRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("no bypass providers found in online catalog")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	updatedCount := 0
	for _, match := range matches {
		rawName := strings.TrimSpace(match[1])
		doh := strings.TrimSpace(match[2])
		fullBlock := match[0]

		var ips []string
		if ipMatch := ipv4Regex.FindStringSubmatch(fullBlock); len(ipMatch) > 1 {
			for _, part := range strings.Split(ipMatch[1], ",") {
				cleanIP := strings.Trim(strings.TrimSpace(part), "'\"")
				if cleanIP != "" {
					ips = append(ips, cleanIP)
				}
			}
		}

		genID := strings.ToLower(regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(rawName, "-"))
		genID = strings.Trim(genID, "-")

		// Check if exists
		foundIdx := -1
		for i, exist := range m.providers {
			if exist.ID == genID || strings.EqualFold(exist.Name, rawName) {
				foundIdx = i
				break
			}
		}

		if foundIdx != -1 {
			// Update endpoint and merge gateways
			existing := m.providers[foundIdx]
			existing.Endpoint = doh
			gwSet := make(map[string]bool)
			for _, g := range existing.Gateways {
				gwSet[g] = true
			}
			for _, ip := range ips {
				if !gwSet[ip] {
					existing.Gateways = append(existing.Gateways, ip)
					gwSet[ip] = true
				}
			}
			m.providers[foundIdx] = existing
			updatedCount++
		} else {
			// Add new provider
			m.providers = append(m.providers, domain.Provider{
				ID:          genID,
				Name:        rawName,
				Kind:        domain.KindDoH,
				Endpoint:    doh,
				Gateways:    ips,
				FallbackIPs: ips,
				Enabled:     true,
				IsCustom:    true,
			})
			updatedCount++
		}
	}

	_ = m.savePersistedLocked()
	return updatedCount, nil
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

func (m *Manager) loadPersisted() {
	data, err := os.ReadFile(m.cfgPath)
	if err != nil {
		return
	}
	var loaded []domain.Provider
	if err := json.Unmarshal(data, &loaded); err == nil && len(loaded) > 0 {
		byId := make(map[string]int)
		for i, p := range m.providers {
			byId[p.ID] = i
		}

		for _, p := range loaded {
			if idx, found := byId[p.ID]; found {
				m.providers[idx] = p
			} else {
				p.IsCustom = true
				m.providers = append(m.providers, p)
			}
		}
	}
}

func (m *Manager) savePersistedLocked() error {
	data, err := json.MarshalIndent(m.providers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.cfgPath, data, 0644)
}

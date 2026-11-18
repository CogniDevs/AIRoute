package hosts

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"airoute/internal/domain"
)

const (
	SectionStartMarker = "# === AIROUTE START"
	SectionEndMarker   = "# === AIROUTE END ==="
)

// Manager safely edits OS hosts files with dedicated markers and privilege elevation.
type Manager struct {
	mu        sync.Mutex
	hostsPath string
}

// NewManager creates a hosts manager using OS-appropriate paths.
func NewManager() *Manager {
	path := "/etc/hosts"
	if runtime.GOOS == "windows" {
		winDir := os.Getenv("SystemRoot")
		if winDir == "" {
			winDir = `C:\Windows`
		}
		path = filepath.Join(winDir, `System32\drivers\etc\hosts`)
	}
	return &Manager{hostsPath: path}
}

// GetHostsPath returns the detected hosts path.
func (m *Manager) GetHostsPath() string {
	return m.hostsPath
}

// ApplyServiceRoutes writes per-service mappings with rich metadata comments.
func (m *Manager) ApplyServiceRoutes(routes map[domain.ServiceTarget]domain.ServiceRoute) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines, err := m.readLines()
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}

	cleanLines := m.stripSection(lines)
	cleanLines = trimTrailingEmptyLines(cleanLines)

	var activeSections []string
	for _, target := range domain.AllServiceTargets {
		route, exists := routes[target]
		if !exists || !route.Enabled || strings.TrimSpace(route.IP) == "" {
			continue
		}

		provName := route.ProviderName
		if provName == "" {
			provName = "Smart DNS"
		}

		var comment string
		if route.LatencyMs > 0 {
			comment = fmt.Sprintf("# [%s] Provider: %s | IP: %s | Latency: %dms", target, provName, route.IP, route.LatencyMs)
		} else {
			comment = fmt.Sprintf("# [%s] Provider: %s | IP: %s", target, provName, route.IP)
		}

		activeSections = append(activeSections, comment)
		for _, d := range domain.TargetDomains[target] {
			activeSections = append(activeSections, fmt.Sprintf("%-16s %s", route.IP, d))
		}
	}

	if len(activeSections) > 0 {
		var newBlock []string
		newBlock = append(newBlock, "\n"+SectionStartMarker+" ===")
		newBlock = append(newBlock, "# Managed by AIRoute. Do not edit manually.")
		newBlock = append(newBlock, activeSections...)
		newBlock = append(newBlock, SectionEndMarker)

		cleanLines = append(cleanLines, newBlock...)
	}

	cleanLines = trimTrailingEmptyLines(cleanLines)
	if err := m.writeLines(cleanLines); err != nil {
		return fmt.Errorf("write hosts: %w", err)
	}

	_ = FlushDNS()
	return nil
}

// ApplyMultiRoute writes target domain mappings with per-service IPs, falling back to defaultIP.
func (m *Manager) ApplyMultiRoute(target domain.ServiceTarget, serviceIPs map[domain.ServiceTarget]string, defaultIP string, providerName string) error {
	routes := make(map[domain.ServiceTarget]domain.ServiceRoute)

	targetsToApply := domain.AllServiceTargets
	if target != "" && target != domain.TargetAll {
		targetsToApply = []domain.ServiceTarget{target}
	}

	for _, t := range targetsToApply {
		ip := ""
		if serviceIPs != nil {
			ip = serviceIPs[t]
		}
		if ip == "" {
			ip = defaultIP
		}
		if ip != "" {
			routes[t] = domain.ServiceRoute{
				Target:       t,
				Enabled:      true,
				ProviderName: providerName,
				IP:           ip,
			}
		}
	}

	return m.ApplyServiceRoutes(routes)
}

// ApplyRoute writes the specified AI target domain mappings using a single clean IP.
func (m *Manager) ApplyRoute(target domain.ServiceTarget, ip string, providerName string) error {
	return m.ApplyMultiRoute(target, nil, ip, providerName)
}

// ClearRoute removes the AIRoute section from the hosts file.
func (m *Manager) ClearRoute() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines, err := m.readLines()
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}

	cleanLines := m.stripSection(lines)
	cleanLines = trimTrailingEmptyLines(cleanLines)

	if err := m.writeLines(cleanLines); err != nil {
		return fmt.Errorf("write hosts: %w", err)
	}

	_ = FlushDNS()
	return nil
}

// GetActiveRoutes parses the hosts file and returns structured per-service route state.
func (m *Manager) GetActiveRoutes() (isActive bool, routes map[domain.ServiceTarget]domain.ServiceRoute, patchedCount int, totalCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	routes = make(map[domain.ServiceTarget]domain.ServiceRoute)
	totalCount = len(domain.AllServiceTargets)
	for _, t := range domain.AllServiceTargets {
		routes[t] = domain.ServiceRoute{
			Target:  t,
			Enabled: false,
		}
	}

	lines, err := m.readLines()
	if err != nil {
		return false, routes, 0, totalCount
	}

	// Reverse lookup: domain -> ServiceTarget
	domainToTarget := make(map[string]domain.ServiceTarget)
	for t, doms := range domain.TargetDomains {
		for _, d := range doms {
			domainToTarget[d] = t
		}
	}

	inSection := false
	var currentTarget domain.ServiceTarget
	var currentProvider string
	var currentLatency int64

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, SectionStartMarker) {
			inSection = true
			continue
		}
		if trimmed == SectionEndMarker {
			break
		}
		if !inSection || len(trimmed) == 0 {
			continue
		}

		// Parse metadata comment: e.g. "# [gemini] Provider: Zapret DNS | IP: 5.129.221.64 | Latency: 38ms"
		if strings.HasPrefix(trimmed, "# [") {
			idxEndBracket := strings.Index(trimmed, "]")
			if idxEndBracket != -1 {
				tgtStr := trimmed[3:idxEndBracket]
				currentTarget = domain.ServiceTarget(tgtStr)
				currentProvider = ""
				currentLatency = 0

				rest := trimmed[idxEndBracket+1:]
				parts := strings.Split(rest, "|")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if strings.HasPrefix(part, "Provider:") {
						currentProvider = strings.TrimSpace(strings.TrimPrefix(part, "Provider:"))
					} else if strings.HasPrefix(part, "Latency:") {
						latStr := strings.TrimSpace(strings.TrimPrefix(part, "Latency:"))
						latStr = strings.TrimSuffix(latStr, "ms")
						if ms, err := strconv.ParseInt(latStr, 10, 64); err == nil {
							currentLatency = ms
						}
					}
				}
			}
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Line format: "<IP> <domain>"
		fields := strings.Fields(trimmed)
		if len(fields) >= 2 {
			ip := fields[0]
			dom := fields[1]

			detectedTarget := currentTarget
			if detectedTarget == "" {
				detectedTarget = domainToTarget[dom]
			}

			if detectedTarget != "" {
				existing := routes[detectedTarget]
				if !existing.Enabled {
					routes[detectedTarget] = domain.ServiceRoute{
						Target:       detectedTarget,
						Enabled:      true,
						ProviderID:   "auto",
						ProviderName: currentProvider,
						IP:           ip,
						LatencyMs:    currentLatency,
					}
				}
			}
		}
	}

	for _, r := range routes {
		if r.Enabled && r.IP != "" {
			patchedCount++
		}
	}
	isActive = patchedCount > 0

	return isActive, routes, patchedCount, totalCount
}

// GetActiveState provides backward-compatible state inspection.
func (m *Manager) GetActiveState() (isActive bool, ip string, target domain.ServiceTarget, provider string) {
	active, routes, patchedCount, _ := m.GetActiveRoutes()
	if !active {
		return false, "", "", ""
	}

	var firstIP, firstProv string
	var enabledTargets []domain.ServiceTarget
	for _, t := range domain.AllServiceTargets {
		r := routes[t]
		if r.Enabled {
			enabledTargets = append(enabledTargets, t)
			if firstIP == "" {
				firstIP = r.IP
				firstProv = r.ProviderName
			}
		}
	}

	if patchedCount == len(domain.AllServiceTargets) {
		target = domain.TargetAll
	} else if len(enabledTargets) == 1 {
		target = enabledTargets[0]
	} else {
		target = domain.TargetAll
	}

	return true, firstIP, target, firstProv
}

func (m *Manager) stripSection(lines []string) []string {
	var out []string
	inSection := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, SectionStartMarker) {
			inSection = true
			continue
		}
		if trimmed == SectionEndMarker {
			inSection = false
			continue
		}
		if !inSection {
			out = append(out, l)
		}
	}
	return out
}

func trimTrailingEmptyLines(lines []string) []string {
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[:end]
}

func (m *Manager) readLines() ([]string, error) {
	f, err := os.Open(m.hostsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func (m *Manager) writeLines(lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	err := os.WriteFile(m.hostsPath, []byte(content), 0644)
	if err == nil {
		return nil
	}

	// Permission denied check
	if isPermissionError(err) {
		tmpFile, tmpErr := os.CreateTemp("", "airoute-hosts-*")
		if tmpErr != nil {
			tmpFile, tmpErr = os.CreateTemp(os.TempDir(), fmt.Sprintf("airoute-hosts-%d.tmp", time.Now().UnixNano()))
			if tmpErr != nil {
				return fmt.Errorf("ошибка создания временного файла: %w", tmpErr)
			}
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, writeErr := tmpFile.WriteString(content); writeErr != nil {
			tmpFile.Close()
			return fmt.Errorf("ошибка записи временного файла: %w", writeErr)
		}
		tmpFile.Close()

		return writeElevated(tmpPath, m.hostsPath)
	}

	return err
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "permission denied") ||
		strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "отказано в доступе")
}

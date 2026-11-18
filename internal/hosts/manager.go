package hosts

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"airoute/internal/domain"
)

const (
	SectionStartMarker = "# === AIROUTE START"
	SectionEndMarker   = "# === AIROUTE END ==="
)

// Manager safely edits OS hosts files with dedicated markers.
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
		path = winDir + `\System32\drivers\etc\hosts`
	}
	return &Manager{hostsPath: path}
}

// GetHostsPath returns the detected hosts path.
func (m *Manager) GetHostsPath() string {
	return m.hostsPath
}

// ApplyMultiRoute writes target domain mappings with per-service IPs, falling back to defaultIP.
func (m *Manager) ApplyMultiRoute(target domain.ServiceTarget, serviceIPs map[domain.ServiceTarget]string, defaultIP string, providerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines, err := m.readLines()
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}

	cleanLines := m.stripSection(lines)

	if target == "" {
		target = domain.TargetAll
	}

	header := fmt.Sprintf("%s: %s (Provider: %s) ===", SectionStartMarker, string(target), providerName)
	var newSection []string
	newSection = append(newSection, "\n"+header)
	newSection = append(newSection, "# Managed by AIRoute. Do not edit manually.")

	services := []domain.ServiceTarget{
		domain.TargetGemini,
		domain.TargetClaude,
		domain.TargetChatGPT,
		domain.TargetAntigravity,
		domain.TargetCursor,
	}

	if target == domain.TargetAll {
		for _, srv := range services {
			ip := ""
			if serviceIPs != nil {
				ip = serviceIPs[srv]
			}
			if ip == "" {
				ip = defaultIP
			}
			if ip == "" {
				continue
			}
			newSection = append(newSection, fmt.Sprintf("# [%s]", srv))
			for _, d := range domain.TargetDomains[srv] {
				newSection = append(newSection, fmt.Sprintf("%-16s %s", ip, d))
			}
		}
	} else {
		ip := ""
		if serviceIPs != nil {
			ip = serviceIPs[target]
		}
		if ip == "" {
			ip = defaultIP
		}
		if ip != "" {
			for _, d := range domain.TargetDomains[target] {
				newSection = append(newSection, fmt.Sprintf("%-16s %s", ip, d))
			}
		}
	}

	newSection = append(newSection, SectionEndMarker+"\n")
	cleanLines = append(cleanLines, newSection...)

	if err := m.writeLines(cleanLines); err != nil {
		return fmt.Errorf("write hosts: %w", err)
	}

	_ = FlushDNS()
	return nil
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

	if err := m.writeLines(cleanLines); err != nil {
		return fmt.Errorf("write hosts: %w", err)
	}

	_ = FlushDNS()
	return nil
}

// GetActiveState inspects the hosts file for an existing AIRoute block.
func (m *Manager) GetActiveState() (isActive bool, ip string, target domain.ServiceTarget, provider string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines, err := m.readLines()
	if err != nil {
		return false, "", "", ""
	}

	target = domain.TargetAll
	inSection := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, SectionStartMarker) {
			inSection = true
			// Header format: "# === AIROUTE START: <target> (Provider: <name>) ==="
			afterMarker := strings.TrimPrefix(trimmed, SectionStartMarker)
			if strings.HasPrefix(afterMarker, ": ") {
				rest := strings.TrimPrefix(afterMarker, ": ")
				if idx := strings.Index(rest, " (Provider: "); idx != -1 {
					parsedTarget := strings.TrimSpace(rest[:idx])
					if parsedTarget != "" {
						target = domain.ServiceTarget(parsedTarget)
					}
					provPart := rest[idx+len(" (Provider: "):]
					provider = strings.TrimSuffix(provPart, ") ===")
				} else {
					parsedTarget := strings.TrimSpace(strings.TrimSuffix(rest, " ==="))
					if parsedTarget != "" {
						target = domain.ServiceTarget(parsedTarget)
					}
				}
			} else if strings.Contains(trimmed, "Provider: ") {
				parts := strings.Split(trimmed, "Provider: ")
				if len(parts) > 1 {
					provider = strings.TrimSuffix(parts[1], ") ===")
				}
			}
			continue
		}
		if trimmed == SectionEndMarker {
			break
		}
		if inSection && len(trimmed) > 0 && !strings.HasPrefix(trimmed, "#") {
			fields := strings.Fields(trimmed)
			if len(fields) >= 2 {
				if ip == "" {
					ip = fields[0]
				}
				isActive = true
			}
		}
	}

	return isActive, ip, target, provider
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
	return os.WriteFile(m.hostsPath, []byte(content), 0644)
}

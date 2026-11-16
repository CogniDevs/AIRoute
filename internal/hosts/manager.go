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

// ApplyRoute writes the specified AI target domain mappings using the clean IP.
func (m *Manager) ApplyRoute(target domain.ServiceTarget, ip string, providerName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lines, err := m.readLines()
	if err != nil {
		return fmt.Errorf("read hosts: %w", err)
	}

	cleanLines := m.stripSection(lines)

	var domains []string
	if target == domain.TargetAll {
		domains = domain.GetAllDomains()
	} else {
		domains = domain.TargetDomains[target]
	}

	header := fmt.Sprintf("%s: %s (Provider: %s) ===", SectionStartMarker, string(target), providerName)
	var newSection []string
	newSection = append(newSection, "\n"+header)
	newSection = append(newSection, "# Managed by AIRoute. Do not edit manually.")

	for _, d := range domains {
		newSection = append(newSection, fmt.Sprintf("%-16s %s", ip, d))
	}
	newSection = append(newSection, SectionEndMarker+"\n")

	cleanLines = append(cleanLines, newSection...)

	if err := m.writeLines(cleanLines); err != nil {
		return fmt.Errorf("write hosts: %w", err)
	}

	_ = FlushDNS()
	return nil
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

	inSection := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, SectionStartMarker) {
			inSection = true
			// Parse header
			if strings.Contains(trimmed, "Provider: ") {
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
				ip = fields[0]
				isActive = true
			}
		}
	}

	return isActive, ip, domain.TargetAll, provider
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

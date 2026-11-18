package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"airoute/internal/domain"
)

func TestHostsManager_ApplyAndClear(t *testing.T) {
	tmpDir := t.TempDir()
	hostsFile := filepath.Join(tmpDir, "hosts")

	initialContent := "127.0.0.1 localhost\n::1 localhost\n192.168.1.10 my-router\n"
	if err := os.WriteFile(hostsFile, []byte(initialContent), 0644); err != nil {
		t.Fatalf("failed to write initial hosts: %v", err)
	}

	mgr := &Manager{hostsPath: hostsFile}

	// 1. Verify initially inactive
	isActive, routes, patchedCount, totalCount := mgr.GetActiveRoutes()
	if isActive || patchedCount != 0 || totalCount != 5 {
		t.Errorf("expected inactive route, got active=%v, patched=%d, total=%d", isActive, patchedCount, totalCount)
	}

	// 2. Apply Route with TargetGemini
	targetIP := "194.87.196.13"
	if err := mgr.ApplyRoute(domain.TargetGemini, targetIP, "Test-DNS"); err != nil {
		t.Fatalf("ApplyRoute failed: %v", err)
	}

	// 3. Verify Active State and correctly parsed per-service route
	isActive, routes, patchedCount, _ = mgr.GetActiveRoutes()
	if !isActive || patchedCount != 1 {
		t.Errorf("expected 1 active route, got active=%v, count=%d", isActive, patchedCount)
	}
	geminiRoute := routes[domain.TargetGemini]
	if !geminiRoute.Enabled || geminiRoute.IP != targetIP {
		t.Errorf("expected gemini route with IP %s, got %+v", targetIP, geminiRoute)
	}

	// Check file content preserves initial lines
	contentBytes, _ := os.ReadFile(hostsFile)
	content := string(contentBytes)
	if !strings.Contains(content, "192.168.1.10 my-router") {
		t.Errorf("user custom hosts entry was lost: %s", content)
	}
	if !strings.Contains(content, targetIP) || !strings.Contains(content, "gemini.google.com") {
		t.Errorf("routed domain missing from hosts: %s", content)
	}

	// 4. Test ApplyServiceRoutes with rich metadata
	serviceRoutes := map[domain.ServiceTarget]domain.ServiceRoute{
		domain.TargetGemini: {
			Target:       domain.TargetGemini,
			Enabled:      true,
			ProviderName: "Zapret DNS",
			IP:           "5.129.221.64",
			LatencyMs:    38,
		},
		domain.TargetClaude: {
			Target:       domain.TargetClaude,
			Enabled:      true,
			ProviderName: "GeoHide",
			IP:           "31.25.239.132",
			LatencyMs:    45,
		},
		domain.TargetChatGPT: {
			Target:       domain.TargetChatGPT,
			Enabled:      false, // Disabled by user
			ProviderName: "MagicDNS",
			IP:           "45.83.27.4",
			LatencyMs:    50,
		},
	}

	if err := mgr.ApplyServiceRoutes(serviceRoutes); err != nil {
		t.Fatalf("ApplyServiceRoutes failed: %v", err)
	}

	isActive, parsedRoutes, patchedCount, _ := mgr.GetActiveRoutes()
	if !isActive || patchedCount != 2 {
		t.Fatalf("expected 2 active patched routes, got %d", patchedCount)
	}

	rGemini := parsedRoutes[domain.TargetGemini]
	if !rGemini.Enabled || rGemini.IP != "5.129.221.64" || rGemini.ProviderName != "Zapret DNS" || rGemini.LatencyMs != 38 {
		t.Errorf("gemini route mismatch: %+v", rGemini)
	}

	rClaude := parsedRoutes[domain.TargetClaude]
	if !rClaude.Enabled || rClaude.IP != "31.25.239.132" || rClaude.ProviderName != "GeoHide" || rClaude.LatencyMs != 45 {
		t.Errorf("claude route mismatch: %+v", rClaude)
	}

	rChatGPT := parsedRoutes[domain.TargetChatGPT]
	if rChatGPT.Enabled {
		t.Errorf("chatgpt was disabled, but got enabled in parsed routes")
	}

	// 5. Test ClearRoute
	if err := mgr.ClearRoute(); err != nil {
		t.Fatalf("ClearRoute failed: %v", err)
	}

	isActive, _, patchedCount, _ = mgr.GetActiveRoutes()
	if isActive || patchedCount != 0 {
		t.Errorf("expected 0 active routes after ClearRoute, got %d", patchedCount)
	}

	contentBytes, _ = os.ReadFile(hostsFile)
	content = string(contentBytes)
	if strings.Contains(content, SectionStartMarker) || strings.Contains(content, SectionEndMarker) {
		t.Errorf("markers remained after ClearRoute: %s", content)
	}
	if !strings.Contains(content, "192.168.1.10 my-router") {
		t.Errorf("user custom hosts entry was lost after ClearRoute: %s", content)
	}
}

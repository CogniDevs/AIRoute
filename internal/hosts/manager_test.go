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
	isActive, ip, _, _ := mgr.GetActiveState()
	if isActive || ip != "" {
		t.Errorf("expected inactive route, got active=%v, ip=%s", isActive, ip)
	}

	// 2. Apply Route with TargetGemini
	targetIP := "194.87.196.13"
	if err := mgr.ApplyRoute(domain.TargetGemini, targetIP, "Test-DNS"); err != nil {
		t.Fatalf("ApplyRoute failed: %v", err)
	}

	// 3. Verify Active State and correctly parsed Target
	isActive, ip, target, prov := mgr.GetActiveState()
	if !isActive {
		t.Errorf("expected active route")
	}
	if ip != targetIP {
		t.Errorf("expected IP %s, got %s", targetIP, ip)
	}
	if prov != "Test-DNS" {
		t.Errorf("expected provider Test-DNS, got %s", prov)
	}
	if target != domain.TargetGemini {
		t.Errorf("expected target %s, got %s", domain.TargetGemini, target)
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

	// 4. Test ApplyMultiRoute with per-service IPs
	multiIPs := map[domain.ServiceTarget]string{
		domain.TargetGemini:  "45.83.27.4",
		domain.TargetClaude:  "5.129.221.64",
		domain.TargetChatGPT: "193.233.112.88",
	}
	if err := mgr.ApplyMultiRoute(domain.TargetAll, multiIPs, "1.1.1.1", "Multi-DNS"); err != nil {
		t.Fatalf("ApplyMultiRoute failed: %v", err)
	}

	isActive, _, target, prov = mgr.GetActiveState()
	if !isActive || target != domain.TargetAll || prov != "Multi-DNS" {
		t.Errorf("ApplyMultiRoute state mismatch: active=%v, target=%s, prov=%s", isActive, target, prov)
	}

	multiContentBytes, _ := os.ReadFile(hostsFile)
	multiContent := string(multiContentBytes)
	if !strings.Contains(multiContent, "45.83.27.4") || !strings.Contains(multiContent, "gemini.google.com") {
		t.Errorf("gemini multi route missing: %s", multiContent)
	}
	if !strings.Contains(multiContent, "5.129.221.64") || !strings.Contains(multiContent, "claude.ai") {
		t.Errorf("claude multi route missing: %s", multiContent)
	}
	if !strings.Contains(multiContent, "193.233.112.88") || !strings.Contains(multiContent, "chatgpt.com") {
		t.Errorf("chatgpt multi route missing: %s", multiContent)
	}

	// 5. Clear Route
	if err := mgr.ClearRoute(); err != nil {
		t.Fatalf("ClearRoute failed: %v", err)
	}

	isActive, _, _, _ = mgr.GetActiveState()
	if isActive {
		t.Errorf("expected inactive after ClearRoute")
	}

	// Check file content reverted cleanly without leaving markers
	contentBytes, _ = os.ReadFile(hostsFile)
	content = string(contentBytes)
	if strings.Contains(content, SectionStartMarker) || strings.Contains(content, SectionEndMarker) {
		t.Errorf("AIRoute markers remained in hosts after clear: %s", content)
	}
	if !strings.Contains(content, "192.168.1.10 my-router") {
		t.Errorf("user custom hosts entry was lost after clear: %s", content)
	}
}

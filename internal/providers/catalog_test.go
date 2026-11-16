package providers

import (
	"path/filepath"
	"testing"

	"airoute/internal/domain"
)

func TestProvidersManager_CustomAddRemove(t *testing.T) {
	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "custom.json")

	mgr := &Manager{
		providers: append([]domain.Provider(nil), defaultProviders...),
		cfgPath:   cfgFile,
	}

	initialLen := len(mgr.GetAll())
	if initialLen == 0 {
		t.Fatal("expected default providers to be loaded")
	}

	// 1. Add custom provider
	customP := domain.Provider{
		ID:       "custom-dns-test",
		Name:     "My Private DoH",
		Kind:     domain.KindDoH,
		Endpoint: "https://my.dns.example/dns-query",
		Enabled:  true,
	}

	if err := mgr.Add(customP); err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	if len(mgr.GetAll()) != initialLen+1 {
		t.Errorf("expected %d providers, got %d", initialLen+1, len(mgr.GetAll()))
	}

	foundP, ok := mgr.GetByID("custom-dns-test")
	if !ok || foundP.Name != "My Private DoH" {
		t.Errorf("custom provider not found or corrupted: %+v", foundP)
	}

	// 2. Remove custom provider
	if err := mgr.Remove("custom-dns-test"); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if len(mgr.GetAll()) != initialLen {
		t.Errorf("expected %d providers after removal, got %d", initialLen, len(mgr.GetAll()))
	}
}

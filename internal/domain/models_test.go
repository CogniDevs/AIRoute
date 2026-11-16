package domain_test

import (
	"testing"

	"airoute/internal/domain"
)

func TestGetAllDomains(t *testing.T) {
	domains := domain.GetAllDomains()
	if len(domains) == 0 {
		t.Fatal("expected non-empty domain list")
	}

	seen := make(map[string]bool)
	for _, d := range domains {
		if seen[d] {
			t.Fatalf("duplicate domain detected: %s", d)
		}
		seen[d] = true
	}

	// Verify key domains are present
	expected := []string{
		"gemini.google.com",
		"claude.ai",
		"chatgpt.com",
		"daily-cloudcode-pa.googleapis.com",
	}

	for _, exp := range expected {
		if !seen[exp] {
			t.Errorf("expected domain %s to be present in GetAllDomains", exp)
		}
	}
}

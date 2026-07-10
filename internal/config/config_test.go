package config

import (
	"os"
	"testing"
	"time"
)

func TestFromEnv(t *testing.T) {
	os.Setenv("BOUNDARY_ADDR", "https://boundary.example.com:9200")
	os.Setenv("BOUNDARY_TOKEN", "at_test123")
	defer os.Unsetenv("BOUNDARY_ADDR")
	defer os.Unsetenv("BOUNDARY_TOKEN")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BoundaryAddr != "https://boundary.example.com:9200" {
		t.Errorf("expected addr, got %s", cfg.BoundaryAddr)
	}
	if cfg.BoundaryToken != "at_test123" {
		t.Errorf("expected token, got %s", cfg.BoundaryToken)
	}
	if cfg.BoundaryClientTimeout != 60*time.Second {
		t.Errorf("expected 60s timeout, got %v", cfg.BoundaryClientTimeout)
	}
	if cfg.BoundaryMaxRetries != 2 {
		t.Errorf("expected 2 retries, got %d", cfg.BoundaryMaxRetries)
	}
}

func TestFromEnvMissingAddr(t *testing.T) {
	os.Unsetenv("BOUNDARY_ADDR")
	os.Unsetenv("BOUNDARY_TOKEN")
	_, err := FromEnv()
	if err == nil {
		t.Error("expected error for missing BOUNDARY_ADDR")
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"at_1234567890", "at_1...7890"},
		{"short", "***"},
		{"", "***"},
	}
	for _, tt := range tests {
		got := MaskToken(tt.input)
		if got != tt.expect {
			t.Errorf("maskToken(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}

func TestHasToken(t *testing.T) {
	c := &Config{BoundaryToken: "at_test"}
	if !c.HasToken() {
		t.Error("expected HasToken true")
	}
	c.BoundaryToken = ""
	if c.HasToken() {
		t.Error("expected HasToken false")
	}
}

func TestTLSInsecure(t *testing.T) {
	os.Setenv("BOUNDARY_ADDR", "https://boundary.example.com")
	os.Setenv("BOUNDARY_TLS_INSECURE", "true")
	defer os.Unsetenv("BOUNDARY_ADDR")
	defer os.Unsetenv("BOUNDARY_TLS_INSECURE")

	cfg, _ := FromEnv()
	if !cfg.BoundaryTLSInsecure {
		t.Error("expected TLS insecure true")
	}
}
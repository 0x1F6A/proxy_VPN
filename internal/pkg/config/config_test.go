package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTP.Addr != ":8080" {
		t.Errorf("expected default http addr :8080, got %q", cfg.HTTP.Addr)
	}
	if cfg.JWT.AccessTTL != 2*time.Hour {
		t.Errorf("expected access ttl 2h, got %s", cfg.JWT.AccessTTL)
	}
	if cfg.App.Env != "test" {
		t.Errorf("expected env from APP_ENV=test, got %q", cfg.App.Env)
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("PROXYVPN_HTTP__ADDR", ":9999")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.HTTP.Addr != ":9999" {
		t.Fatalf("env override failed, got %q", cfg.HTTP.Addr)
	}
}

package unit_tests

import (
	"os"
	"testing"

	"careops/clinic/internal/config"
)

func TestConfig_Defaults(t *testing.T) {
	// Unset all env vars to exercise defaults
	vars := []string{
		"PORT", "DB_PATH", "MIGRATIONS_PATH", "SEEDS_PATH",
		"SESSION_TTL_HOURS", "COOKIE_NAME", "LOG_LEVEL", "LOG_FORMAT", "APP_ENV",
	}
	for _, v := range vars {
		os.Unsetenv(v)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("expected Port=8080, got %q", cfg.Port)
	}
	if cfg.SessionTTLHours != 24 {
		t.Errorf("expected SessionTTLHours=24, got %d", cfg.SessionTTLHours)
	}
	if cfg.CookieName != "careops_session" {
		t.Errorf("expected CookieName=careops_session, got %q", cfg.CookieName)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel=info, got %q", cfg.LogLevel)
	}
}

func TestConfig_EnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("SESSION_TTL_HOURS", "12")
	t.Setenv("APP_ENV", "development")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected Port=9090, got %q", cfg.Port)
	}
	if cfg.SessionTTLHours != 12 {
		t.Errorf("expected SessionTTLHours=12, got %d", cfg.SessionTTLHours)
	}
	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %q", cfg.Env)
	}
}

func TestConfig_InvalidTTL(t *testing.T) {
	t.Setenv("SESSION_TTL_HOURS", "not-a-number")
	_, err := config.Load()
	if err == nil {
		t.Error("expected error for invalid SESSION_TTL_HOURS, got nil")
	}
}

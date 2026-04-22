package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration, driven entirely by environment variables.
// Every field has a sane default so the binary runs correctly inside the Docker image
// without requiring explicit env overrides.
type Config struct {
	// Server
	Port string

	// Storage paths (relative for dev; absolute inside Docker image)
	DBPath          string
	MigrationsPath  string
	SeedsPath       string
	TemplatesPath   string
	StaticPath      string
	MediaPath       string
	ExportsPath     string
	DiagnosticsPath string
	ConfigSnapPath  string

	// Auth
	SessionTTLHours           int
	InactivityTimeoutMinutes  int
	CookieName                string

	// Field encryption: AES-256-GCM key for sensitive columns.
	// See internal/shared/crypto for key format and loading docs.
	EncryptKeyPath string

	// FacilityTimezone controls how exam window times are interpreted and how
	// datetimes are displayed to staff. Must be a valid IANA name.
	// All stored datetimes remain UTC; this setting only affects localisation.
	FacilityTimezone string

	// Logging
	LogLevel  string // "debug" | "info" | "warn" | "error"
	LogFormat string // "json" | "text"
	LogPath   string // optional file path; when set, logs are written to both stdout and this file

	// Environment label — used in page headers and log fields
	Env string // "production" | "development"
}

func Load() (*Config, error) {
	sessionTTL, err := parseInt(env("SESSION_TTL_HOURS", "24"))
	if err != nil {
		return nil, fmt.Errorf("config: SESSION_TTL_HOURS: %w", err)
	}
	inactivityTimeout, err := parseInt(env("INACTIVITY_TIMEOUT_MINUTES", "15"))
	if err != nil {
		return nil, fmt.Errorf("config: INACTIVITY_TIMEOUT_MINUTES: %w", err)
	}

	return &Config{
		Port:                     env("PORT", "8080"),
		DBPath:                   env("DB_PATH", "./data/db/careops.db"),
		MigrationsPath:           env("MIGRATIONS_PATH", "./migrations"),
		SeedsPath:                env("SEEDS_PATH", "./seeds"),
		TemplatesPath:            env("TEMPLATES_PATH", "./web/templates"),
		StaticPath:               env("STATIC_PATH", "./web/static"),
		MediaPath:                env("MEDIA_PATH", "./data/media"),
		ExportsPath:              env("EXPORTS_PATH", "./data/exports"),
		DiagnosticsPath:          env("DIAGNOSTICS_PATH", "./data/diagnostics"),
		ConfigSnapPath:           env("CONFIG_SNAP_PATH", "./data/config_snapshots"),
		SessionTTLHours:          sessionTTL,
		InactivityTimeoutMinutes: inactivityTimeout,
		CookieName:               env("COOKIE_NAME", "careops_session"),
		EncryptKeyPath:           env("FIELD_ENCRYPT_KEY_FILE", ""),
		FacilityTimezone:         env("FACILITY_TIMEZONE", "America/New_York"),
		LogLevel:                 env("LOG_LEVEL", "info"),
		LogFormat:                env("LOG_FORMAT", "json"),
		LogPath:                  env("LOG_PATH", "./data/logs/app.log"),
		Env:                      env("APP_ENV", "production"),
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("expected integer, got %q", s)
	}
	return n, nil
}

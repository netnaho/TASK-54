package api_tests

import (
	"os"
	"testing"

	appmod "careops/clinic/internal/app"
	"careops/clinic/internal/config"
	"careops/clinic/internal/platform/logger"
)

// TestProductionStartup_FailsWithoutEncryptionKey verifies that
// ValidateProductionConfig returns an error in production mode when no
// encryption key environment variable is configured.
func TestProductionStartup_FailsWithoutEncryptionKey(t *testing.T) {
	// Ensure no key is set.
	os.Unsetenv("FIELD_ENCRYPT_KEY")
	os.Unsetenv("FIELD_ENCRYPT_KEY_FILE")

	cfg := &config.Config{Env: "production"}
	log := logger.New("text", "error")

	err := appmod.ValidateProductionConfig(cfg, log)
	if err == nil {
		t.Error("ValidateProductionConfig: want error in production mode without key, got nil")
	}
}

// TestProductionStartup_SucceedsInDevelopmentWithoutKey verifies that
// ValidateProductionConfig returns nil (just logs a warning) in development mode
// even when no encryption key is configured.
func TestProductionStartup_SucceedsInDevelopmentWithoutKey(t *testing.T) {
	os.Unsetenv("FIELD_ENCRYPT_KEY")
	os.Unsetenv("FIELD_ENCRYPT_KEY_FILE")

	cfg := &config.Config{Env: "development"}
	log := logger.New("text", "error")

	err := appmod.ValidateProductionConfig(cfg, log)
	if err != nil {
		t.Errorf("ValidateProductionConfig: want nil in development mode without key, got: %v", err)
	}
}

// TestProductionStartup_SucceedsWithValidKey verifies that ValidateProductionConfig
// returns nil in production mode when a valid 64-char hex key is provided.
func TestProductionStartup_SucceedsWithValidKey(t *testing.T) {
	validKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("FIELD_ENCRYPT_KEY", validKey)
	defer os.Unsetenv("FIELD_ENCRYPT_KEY")

	cfg := &config.Config{Env: "production"}
	log := logger.New("text", "error")

	err := appmod.ValidateProductionConfig(cfg, log)
	if err != nil {
		t.Errorf("ValidateProductionConfig with valid key: want nil, got: %v", err)
	}
}

// TestProductionStartup_InvalidKeyFailsInProduction verifies that an invalid
// (non-hex, wrong length) key causes ValidateProductionConfig to return an error
// in production mode.
func TestProductionStartup_InvalidKeyFailsInProduction(t *testing.T) {
	t.Setenv("FIELD_ENCRYPT_KEY", "not-a-valid-hex-key")
	defer os.Unsetenv("FIELD_ENCRYPT_KEY")

	cfg := &config.Config{Env: "production"}
	log := logger.New("text", "error")

	err := appmod.ValidateProductionConfig(cfg, log)
	if err == nil {
		t.Error("ValidateProductionConfig: want error for invalid key in production, got nil")
	}
}

// TestProductionStartup_InvalidKeySucceedsInDevelopment verifies that an invalid
// key only logs a warning (does not fail) in development mode.
func TestProductionStartup_InvalidKeySucceedsInDevelopment(t *testing.T) {
	t.Setenv("FIELD_ENCRYPT_KEY", "not-a-valid-hex-key")
	defer os.Unsetenv("FIELD_ENCRYPT_KEY")

	cfg := &config.Config{Env: "development"}
	log := logger.New("text", "error")

	err := appmod.ValidateProductionConfig(cfg, log)
	if err != nil {
		t.Errorf("ValidateProductionConfig invalid key in development: want nil, got: %v", err)
	}
}

package app

import (
	"fmt"
	"log/slog"

	"careops/clinic/internal/config"
	"careops/clinic/internal/shared/crypto"
)

// ValidateProductionConfig checks that all production-required secrets are
// configured. In development mode it logs a warning and returns nil so the
// binary still starts for local development and tests.
//
// Must be called after Bootstrap, before http.Listen.
func ValidateProductionConfig(cfg *config.Config, log *slog.Logger) error {
	cipher, err := crypto.LoadFromEnv()
	if err != nil {
		if cfg.Env == "production" {
			return fmt.Errorf("production: field encryption key is invalid: %w", err)
		}
		log.Warn("field encryption key is set but invalid; continuing in development mode", "err", err)
		return nil
	}

	if cipher == nil {
		if cfg.Env == "production" {
			return fmt.Errorf(
				"production: FIELD_ENCRYPT_KEY or FIELD_ENCRYPT_KEY_FILE must be configured — " +
					"sensitive payment reference numbers cannot be stored without encryption in production mode")
		}
		log.Warn("field encryption not configured; sensitive payment references will be stored unencrypted (development mode only)")
	}
	return nil
}

package app

import (
	"fmt"
	"log/slog"

	"careops/clinic/internal/config"
	"careops/clinic/internal/platform/database"
	dbsql "database/sql"
)

// Bootstrap opens the database, runs migrations, and runs seeds.
// It returns the open *sql.DB, ready for use by the application.
// No traffic is served until Bootstrap returns without error.
func Bootstrap(cfg *config.Config, log *slog.Logger) (*dbsql.DB, error) {
	return bootstrap(cfg, log, database.Open)
}

// BootstrapFast opens the database with synchronous=OFF for use in tests.
// Not safe for production — intended only for test environments.
func BootstrapFast(cfg *config.Config, log *slog.Logger) (*dbsql.DB, error) {
	return bootstrap(cfg, log, database.OpenFast)
}

func bootstrap(cfg *config.Config, log *slog.Logger, open func(string) (*dbsql.DB, error)) (*dbsql.DB, error) {
	db, err := open(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: open db: %w", err)
	}

	log.Info("running migrations", "path", cfg.MigrationsPath)
	if err := database.Migrate(db, cfg.MigrationsPath, log); err != nil {
		return nil, fmt.Errorf("bootstrap: migrate: %w", err)
	}

	log.Info("running seeds", "path", cfg.SeedsPath)
	if err := database.Seed(db, cfg.SeedsPath, log); err != nil {
		return nil, fmt.Errorf("bootstrap: seed: %w", err)
	}

	return db, nil
}

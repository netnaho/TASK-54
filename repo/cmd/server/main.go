package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"careops/clinic/internal/app"
	"careops/clinic/internal/config"
	"careops/clinic/internal/platform/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	if cfg.LogPath != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.LogPath), 0o750); err != nil {
			slog.Warn("could not create log directory", "path", cfg.LogPath, "err", err)
		}
	}
	log, logCloser := logger.NewDual(cfg.LogFormat, cfg.LogLevel, cfg.LogPath)
	defer logCloser.Close()
	log.Info("starting CareOps", "env", cfg.Env, "port", cfg.Port)

	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		log.Error("bootstrap failed", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := app.ValidateProductionConfig(cfg, log); err != nil {
		log.Error("startup validation failed", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	app.StartReportScheduler(ctx, db, cfg, log)

	fiberApp := app.New(cfg, db, log)

	log.Info("server listening", "addr", ":"+cfg.Port)
	if err := fiberApp.Listen(":" + cfg.Port); err != nil {
		log.Error("server error", "err", err)
		os.Exit(1)
	}
}

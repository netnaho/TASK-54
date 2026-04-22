package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Migrate scans migrationsPath for *.sql files ordered by filename, applies any that
// have not yet been recorded in schema_migrations, and records each one atomically.
// Re-running Migrate is always safe — already-applied versions are skipped.
func Migrate(db *sql.DB, migrationsPath string, log *slog.Logger) error {
	if err := ensureMigrationsTable(db); err != nil {
		return err
	}

	files, err := sqlFilesInDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("migrate: list %s: %w", migrationsPath, err)
	}

	applied, err := appliedVersions(db, "schema_migrations")
	if err != nil {
		return fmt.Errorf("migrate: read applied: %w", err)
	}

	for _, f := range files {
		version := strings.TrimSuffix(filepath.Base(f), ".sql")
		if applied[version] {
			log.Debug("migration already applied", "version", version)
			continue
		}

		content, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("migrate: read %s: %w", f, err)
		}

		if err := applyScript(db, version, string(content), "schema_migrations"); err != nil {
			return fmt.Errorf("migrate: apply %s: %w", version, err)
		}
		log.Info("migration applied", "version", version)
	}

	return nil
}

// Seed scans seedsPath for *.sql files and applies any not yet in schema_seeds.
func Seed(db *sql.DB, seedsPath string, log *slog.Logger) error {
	if err := ensureSeedsTable(db); err != nil {
		return err
	}

	files, err := sqlFilesInDir(seedsPath)
	if err != nil {
		return fmt.Errorf("seed: list %s: %w", seedsPath, err)
	}

	applied, err := appliedVersions(db, "schema_seeds")
	if err != nil {
		return fmt.Errorf("seed: read applied: %w", err)
	}

	for _, f := range files {
		version := strings.TrimSuffix(filepath.Base(f), ".sql")
		if applied[version] {
			log.Debug("seed already applied", "version", version)
			continue
		}

		content, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("seed: read %s: %w", f, err)
		}

		if err := applyScript(db, version, string(content), "schema_seeds"); err != nil {
			return fmt.Errorf("seed: apply %s: %w", version, err)
		}
		log.Info("seed applied", "version", version)
	}

	return nil
}

// — helpers —

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT NOT NULL PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	return err
}

func ensureSeedsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_seeds (
		version    TEXT NOT NULL PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	return err
}

func appliedVersions(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf("SELECT version FROM %s", table)) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		m[v] = true
	}
	return m, rows.Err()
}

func applyScript(db *sql.DB, version, sql, trackingTable string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	if _, err := tx.Exec(sql); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("execute script: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(
		fmt.Sprintf("INSERT INTO %s (version, applied_at) VALUES (?, ?)", trackingTable), //nolint:gosec
		version, now,
	); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record version: %w", err)
	}

	return tx.Commit()
}

func sqlFilesInDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	return files, nil
}

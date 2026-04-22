package unit_tests

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"careops/clinic/internal/platform/database"
	"careops/clinic/internal/platform/logger"
)

func repoRoot() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..")
}

func TestMigrator_AppliesAndTracksVersions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	log := logger.New("text", "error")
	migrationsPath := filepath.Join(repoRoot(), "migrations")

	// First run — should apply all migrations
	if err := database.Migrate(db, migrationsPath, log); err != nil {
		t.Fatalf("Migrate (first run): %v", err)
	}

	// Verify schema_migrations has entries
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if count == 0 {
		t.Error("expected at least one migration recorded, got 0")
	}

	// Second run — must be idempotent (no error, no duplicate rows)
	if err := database.Migrate(db, migrationsPath, log); err != nil {
		t.Fatalf("Migrate (second run): %v", err)
	}

	var count2 int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count2); err != nil {
		t.Fatalf("query schema_migrations (2nd run): %v", err)
	}
	if count2 != count {
		t.Errorf("migration count changed on second run: %d → %d", count, count2)
	}
}

func TestSeeder_AppliesAndIsIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	log := logger.New("text", "error")
	migrationsPath := filepath.Join(repoRoot(), "migrations")
	seedsPath := filepath.Join(repoRoot(), "seeds")

	if err := database.Migrate(db, migrationsPath, log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if err := database.Seed(db, seedsPath, log); err != nil {
		t.Fatalf("Seed (first run): %v", err)
	}

	// Verify some seed data landed
	var userCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount); err != nil {
		t.Fatalf("query users: %v", err)
	}
	if userCount == 0 {
		t.Error("expected seed users, got 0")
	}

	// Second run — idempotent
	if err := database.Seed(db, seedsPath, log); err != nil {
		t.Fatalf("Seed (second run): %v", err)
	}

	var userCount2 int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&userCount2); err != nil {
		t.Fatalf("query users (2nd): %v", err)
	}
	if userCount2 != userCount {
		t.Errorf("user count changed after re-seed: %d → %d", userCount, userCount2)
	}

	// Verify bcrypt hash is non-empty for admin user
	var hash string
	if err := db.QueryRow(`SELECT password_hash FROM users WHERE email = 'admin@careops.local'`).Scan(&hash); err != nil {
		t.Fatalf("query admin hash: %v", err)
	}
	if len(hash) < 20 {
		t.Errorf("expected bcrypt hash, got %q", hash)
	}
}

// Ensure the test file exists even when run without OS environment
var _ = os.DevNull

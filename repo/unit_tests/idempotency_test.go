package unit_tests

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"careops/clinic/internal/platform/database"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/platform/logger"
)

func newMigratedDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	log := logger.New("text", "error")
	if err := database.Migrate(db, filepath.Join(repoRoot(), "migrations"), log); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

func TestIdempotency_CheckReturnsNilForUnknownKey(t *testing.T) {
	db := newMigratedDB(t)
	svc := idempotency.NewService(db)

	entry, err := svc.Check("nonexistent-key-xyz")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if entry != nil {
		t.Error("expected nil entry for unknown key")
	}
}

func TestIdempotency_StoreAndReplay(t *testing.T) {
	db := newMigratedDB(t)
	svc := idempotency.NewService(db)

	key := "idem-store-001"
	if err := svc.Store(key, 201, `{"id":"abc"}`); err != nil {
		t.Fatalf("Store: %v", err)
	}

	entry, err := svc.Check(key)
	if err != nil {
		t.Fatalf("Check after Store: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if entry.ResponseStatus != 201 {
		t.Errorf("status: want 201, got %d", entry.ResponseStatus)
	}
	if entry.ResponseBody != `{"id":"abc"}` {
		t.Errorf("body: want %q, got %q", `{"id":"abc"}`, entry.ResponseBody)
	}
}

func TestIdempotency_DuplicateKeyPreservesOriginal(t *testing.T) {
	db := newMigratedDB(t)
	svc := idempotency.NewService(db)

	key := "idem-dup-001"
	if err := svc.Store(key, 200, "first-response"); err != nil {
		t.Fatalf("Store first: %v", err)
	}
	// Second Store on same key must be a no-op (ON CONFLICT DO NOTHING).
	if err := svc.Store(key, 500, "second-response"); err != nil {
		t.Fatalf("Store second: %v", err)
	}

	entry, err := svc.Check(key)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.ResponseStatus != 200 || entry.ResponseBody != "first-response" {
		t.Errorf("duplicate store overwrote original: status=%d body=%q",
			entry.ResponseStatus, entry.ResponseBody)
	}
}

func TestIdempotency_ExpiredKeyTreatedAsNew(t *testing.T) {
	db := newMigratedDB(t)
	svc := idempotency.NewService(db)

	key := "idem-expired-001"
	past := time.Now().UTC().Add(-25 * time.Hour).Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO idempotency_keys (key, response_status, response_body, created_at, expires_at)
		VALUES (?, 200, 'stale', ?, ?)`, key, past, past)
	if err != nil {
		t.Fatalf("direct insert: %v", err)
	}

	entry, err := svc.Check(key)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if entry != nil {
		t.Error("expected nil for expired key, got an entry")
	}

	// After expiry cleanup, re-storing must succeed.
	if err := svc.Store(key, 201, "fresh"); err != nil {
		t.Fatalf("Store after expiry: %v", err)
	}
	fresh, _ := svc.Check(key)
	if fresh == nil || fresh.ResponseBody != "fresh" {
		t.Error("expected fresh entry after expired key was cleared")
	}
}

func TestIdempotency_CleanupRemovesOnlyExpiredEntries(t *testing.T) {
	db := newMigratedDB(t)
	svc := idempotency.NewService(db)

	past := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339)
	for _, key := range []string{"old-1", "old-2"} {
		if _, err := db.Exec(`
			INSERT INTO idempotency_keys (key, response_status, response_body, created_at, expires_at)
			VALUES (?, 200, '', ?, ?)`, key, past, past); err != nil {
			t.Fatalf("insert expired key: %v", err)
		}
	}
	if err := svc.Store("live-key", 200, ""); err != nil {
		t.Fatalf("Store live: %v", err)
	}

	n, err := svc.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup: %v", err)
	}
	if n != 2 {
		t.Errorf("expected 2 expired rows deleted, got %d", n)
	}

	live, _ := svc.Check("live-key")
	if live == nil {
		t.Error("live key was incorrectly deleted by cleanup")
	}
}

func TestIdempotency_TTLIs24Hours(t *testing.T) {
	db := newMigratedDB(t)
	svc := idempotency.NewService(db)

	if err := svc.Store("ttl-check", 200, ""); err != nil {
		t.Fatalf("Store: %v", err)
	}

	var expiresAt string
	if err := db.QueryRow(
		`SELECT expires_at FROM idempotency_keys WHERE key = 'ttl-check'`,
	).Scan(&expiresAt); err != nil {
		t.Fatalf("QueryRow: %v", err)
	}
	exp, _ := time.Parse(time.RFC3339, expiresAt)
	diff := exp.Sub(time.Now().UTC())
	if diff < 23*time.Hour || diff > 25*time.Hour {
		t.Errorf("expected ~24h TTL, got %v", diff)
	}
}

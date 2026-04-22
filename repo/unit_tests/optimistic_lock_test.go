package unit_tests

import (
	"database/sql"
	"errors"
	"testing"

	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/dbutil"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// insertTestBed inserts a minimal bed row with row_version=1 and returns its ID.
func insertTestBed(t *testing.T, db *sql.DB) string {
	t.Helper()
	id := idgen.New()
	now := timeutil.NowISO()
	_, err := db.Exec(`
		INSERT INTO beds (id, ward, room_number, bed_label, bed_type, is_active, row_version, created_at, updated_at)
		VALUES (?, 'Test Wing', '99', 'X', 'standard', 1, 1, ?, ?)`,
		id, now, now,
	)
	if err != nil {
		t.Fatalf("insertTestBed: %v", err)
	}
	return id
}

func TestCheckRowVersion_MatchSucceeds(t *testing.T) {
	db := newMigratedDB(t)
	id := insertTestBed(t, db)

	if err := dbutil.CheckRowVersion(db, "beds", id, 1); err != nil {
		t.Errorf("expected no error for matching version, got: %v", err)
	}
}

func TestCheckRowVersion_MismatchReturnsConflict(t *testing.T) {
	db := newMigratedDB(t)
	id := insertTestBed(t, db)

	err := dbutil.CheckRowVersion(db, "beds", id, 99) // caller thinks version is 99, actual is 1
	if err == nil {
		t.Fatal("expected conflict error, got nil")
	}

	var ce *apperr.ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("expected *apperr.ConflictError, got %T: %v", err, err)
	}
	if ce.CurrentVersion != 1 {
		t.Errorf("expected current version 1, got %d", ce.CurrentVersion)
	}
	if !errors.Is(err, apperr.ErrConflict) {
		t.Error("expected errors.Is(err, ErrConflict) to be true")
	}
}

func TestCheckRowVersion_NotFoundReturnsConflict(t *testing.T) {
	db := newMigratedDB(t)

	err := dbutil.CheckRowVersion(db, "beds", "nonexistent-uuid-00000", 1)
	if err == nil {
		t.Fatal("expected conflict error for non-existent row, got nil")
	}
	if !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("expected ErrConflict, got: %v", err)
	}
}

func TestConflictError_Unwrap(t *testing.T) {
	ce := &apperr.ConflictError{Entity: "beds", ID: "abc", CurrentVersion: 3}
	if !errors.Is(ce, apperr.ErrConflict) {
		t.Error("ConflictError should unwrap to ErrConflict via Unwrap()")
	}
	if ce.Error() == "" {
		t.Error("ConflictError.Error() should return non-empty string")
	}
}

func TestVersionIncrementedOnUpdate(t *testing.T) {
	db := newMigratedDB(t)
	id := insertTestBed(t, db)

	// Simulate a write path: check version, then update with increment.
	if err := dbutil.CheckRowVersion(db, "beds", id, 1); err != nil {
		t.Fatalf("CheckRowVersion: %v", err)
	}
	_, err := db.Exec(
		`UPDATE beds SET bed_label = 'Y', row_version = row_version + 1, updated_at = ? WHERE id = ?`,
		timeutil.NowISO(), id,
	)
	if err != nil {
		t.Fatalf("UPDATE: %v", err)
	}

	// Now version is 2; a stale caller expecting 1 should get a conflict.
	if err := dbutil.CheckRowVersion(db, "beds", id, 1); !errors.Is(err, apperr.ErrConflict) {
		t.Error("expected conflict after version increment, but CheckRowVersion passed")
	}
	// A caller with the current version (2) should succeed.
	if err := dbutil.CheckRowVersion(db, "beds", id, 2); err != nil {
		t.Errorf("expected success for current version 2, got: %v", err)
	}
}

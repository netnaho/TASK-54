// Package dbutil provides database utility helpers shared across modules.
package dbutil

import (
	"database/sql"
	"fmt"

	"careops/clinic/internal/shared/apperr"
)

// CheckRowVersion verifies that the row identified by (table, id) still has
// the expectedVersion. Returns *apperr.ConflictError if versions diverge,
// so callers can detect stale-update attempts without implementing the check
// themselves.
//
// IMPORTANT: table must be a compile-time constant from your own code, never
// a user-supplied string — SQLite does not support parameterised table names.
func CheckRowVersion(db *sql.DB, table, id string, expectedVersion int) error {
	var current int
	err := db.QueryRow(
		fmt.Sprintf("SELECT row_version FROM %s WHERE id = ?", table), id,
	).Scan(&current)

	if err == sql.ErrNoRows {
		return &apperr.ConflictError{Entity: table, ID: id, CurrentVersion: 0}
	}
	if err != nil {
		return fmt.Errorf("dbutil: check row version for %s/%s: %w", table, id, err)
	}
	if current != expectedVersion {
		return &apperr.ConflictError{Entity: table, ID: id, CurrentVersion: current}
	}
	return nil
}

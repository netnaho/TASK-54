package unit_tests

import (
	"database/sql"
	"os"
	"testing"

	"careops/clinic/internal/platform/database"
	_ "modernc.org/sqlite"
)

func TestLiveOccupancyCount(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/occ_test.db"

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer os.Remove(dbPath)
	defer db.Close()

	// Minimal schema
	_, err = db.Exec(`
		CREATE TABLE beds (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'available');
		CREATE TABLE admissions (
			id TEXT PRIMARY KEY,
			resident_id TEXT NOT NULL,
			bed_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			discharge_date TEXT
		);
		INSERT INTO beds VALUES ('b1','available'),('b2','available'),('b3','available');
		INSERT INTO admissions VALUES
			('a1','r1','b1','active',NULL),
			('a2','r2','b2','active',NULL),
			('a3','r3','b3','discharged','2024-01-10');
	`)
	if err != nil {
		t.Fatalf("setup schema: %v", err)
	}

	var occupied int
	err = db.QueryRow(`SELECT COUNT(*) FROM admissions WHERE status='active' AND discharge_date IS NULL`).Scan(&occupied)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if occupied != 2 {
		t.Errorf("expected 2 active admissions, got %d", occupied)
	}
}

func TestLiveOccupancyZeroWhenNoneAdmitted(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/occ_empty.db"

	db, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer os.Remove(dbPath)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE admissions (
			id TEXT PRIMARY KEY,
			resident_id TEXT NOT NULL,
			bed_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			discharge_date TEXT
		);
	`)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	var occupied int
	err = db.QueryRow(`SELECT COUNT(*) FROM admissions WHERE status='active' AND discharge_date IS NULL`).Scan(&occupied)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if occupied != 0 {
		t.Errorf("expected 0, got %d", occupied)
	}
}

// Ensure database.Open is available (compilation check)
var _ *sql.DB

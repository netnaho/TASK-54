package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO required
)

// Open creates (if necessary) the SQLite file at dbPath and returns a configured *sql.DB.
// WAL mode, foreign key enforcement, and a busy timeout are set pragmatically on every
// connection via the DSN so they survive connection pool recycling.
func Open(dbPath string) (*sql.DB, error) {
	return openWithSync(dbPath, "NORMAL")
}

// OpenFast opens a SQLite database with synchronous=OFF and WAL journal mode.
// Intended for test environments only — data loss on crash is acceptable.
func OpenFast(dbPath string) (*sql.DB, error) {
	return openWithSync(dbPath, "OFF")
}

func openWithSync(dbPath, syncMode string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("database: create directory for %s: %w", dbPath, err)
	}

	dsn := fmt.Sprintf(
		"file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)&_pragma=synchronous(%s)",
		dbPath, syncMode,
	)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: open %s: %w", dbPath, err)
	}

	// WAL mode allows concurrent readers; serialise only writers.
	// Setting MaxOpenConns > 1 is safe for reads; the busy_timeout pragma handles
	// write-write contention without deadlocking.
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("database: ping %s: %w", dbPath, err)
	}

	return db, nil
}

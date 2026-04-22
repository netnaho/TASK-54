// Package jobs provides a fire-and-forget writer for the job_history table.
// Every significant background or on-demand operation (batch imports, shift
// closes, report runs, diagnostic exports) records a row here so operators
// have a single place to review execution history and failure root causes.
package jobs

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/shared/idgen"
)

// Status values mirror the job_history.status CHECK comment in the schema.
const (
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusSkipped = "skipped"
)

// Entry describes a single job execution to persist.
type Entry struct {
	JobName       string
	JobType       string // "scheduled" | "on_demand"
	Status        string
	DurationMs    int64
	Detail        string
	Actor         string // user_id or "system"
	CorrelationID string // request_id from HTMX flow, if available
}

// Writer appends job_history rows. The zero value is not usable; use NewWriter.
type Writer struct {
	db *sql.DB
}

// NewWriter returns a Writer backed by the given database connection.
func NewWriter(db *sql.DB) *Writer {
	return &Writer{db: db}
}

// Record inserts a job execution entry. Errors are non-fatal — a failed write
// must never block the operation that triggered the job.
func (w *Writer) Record(e Entry) error {
	id := idgen.New()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := w.db.Exec(`
		INSERT INTO job_history
		    (id, job_name, job_type, status, duration_ms, detail,
		     actor, correlation_id, ran_at, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, e.JobName, e.JobType, e.Status, e.DurationMs, e.Detail,
		e.Actor, e.CorrelationID, now, now,
	)
	if err != nil {
		return fmt.Errorf("jobs.Record: %w", err)
	}
	return nil
}

// Timed is a convenience helper that records the duration automatically.
// Usage:
//
//	done := jobWriter.Timed(entry); defer done(err)
func (w *Writer) Timed(e Entry) func(errPtr *error) {
	start := time.Now()
	return func(errPtr *error) {
		e.DurationMs = time.Since(start).Milliseconds()
		if errPtr != nil && *errPtr != nil {
			e.Status = StatusFailed
			e.Detail = fmt.Sprintf("FAILED: %v — %s", *errPtr, e.Detail)
		}
		_ = w.Record(e) // non-fatal
	}
}

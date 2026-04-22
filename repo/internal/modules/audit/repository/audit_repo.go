package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"careops/clinic/internal/modules/audit/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

type AuditRepo struct {
	db *sql.DB
}

func NewAuditRepo(db *sql.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

// RecordEntry is the write-input DTO for audit log entries.
// Callers must mask sensitive field values before populating Details,
// BeforeSnapshot, or AfterSnapshot (see crypto.MaskString).
type RecordEntry struct {
	UserID         string
	Action         string
	EntityType     string
	EntityID       string
	Details        string // JSON object; defaults to "{}" if empty
	BeforeSnapshot string // JSON of prior state; "" if not applicable
	AfterSnapshot  string // JSON of new state; "" if not applicable
	IPAddress      string
	RequestID      string
	Outcome        string // "success" | "failure" | "error"; defaults to "success"
}

// Record inserts a new audit log entry.
// This is intentionally write-only at the application layer — no UPDATE or DELETE
// paths exist for audit_logs; the table is append-only by design.
func (r *AuditRepo) Record(e RecordEntry) error {
	if e.Details == "" {
		e.Details = "{}"
	}
	if e.Outcome == "" {
		e.Outcome = "success"
	}
	now := timeutil.NowISO()
	_, err := r.db.Exec(`
		INSERT INTO audit_logs
		  (id, user_id, action, entity_type, entity_id, details,
		   before_snapshot, after_snapshot, outcome,
		   ip_address, request_id, occurred_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		idgen.New(),
		e.UserID, e.Action, e.EntityType, e.EntityID, e.Details,
		e.BeforeSnapshot, e.AfterSnapshot, e.Outcome,
		e.IPAddress, e.RequestID, now, now,
	)
	return err
}

// List returns the most recent audit log entries, newest first.
// Use ListFiltered for query-based lookups.
func (r *AuditRepo) List(limit int) ([]domain.AuditLog, error) {
	return r.ListFiltered(domain.ListFilter{Limit: limit})
}

// ListFiltered returns audit entries matching the provided filter criteria.
// All filter fields are optional; an empty filter returns all entries up to Limit.
func (r *AuditRepo) ListFiltered(f domain.ListFilter) ([]domain.AuditLog, error) {
	if f.Limit <= 0 {
		f.Limit = 200
	}

	var args []any
	var where []string

	if f.EntityType != "" {
		where = append(where, "a.entity_type = ?")
		args = append(args, f.EntityType)
	}
	if f.EntityID != "" {
		where = append(where, "a.entity_id = ?")
		args = append(args, f.EntityID)
	}
	if f.UserID != "" {
		where = append(where, "a.user_id = ?")
		args = append(args, f.UserID)
	}
	if f.DateFrom != "" {
		where = append(where, "a.occurred_at >= ?")
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		where = append(where, "a.occurred_at <= ?")
		args = append(args, f.DateTo+"T23:59:59Z")
	}
	if f.ResidentID != "" {
		// Matches audit entries whose entity_id belongs to a resident-linked record.
		where = append(where, `(
			(a.entity_type = 'payments'       AND a.entity_id IN (SELECT id FROM payments       WHERE resident_id = ?))
			OR (a.entity_type = 'service_orders' AND a.entity_id IN (SELECT id FROM service_orders WHERE resident_id = ?))
			OR (a.entity_type = 'admissions'     AND a.entity_id IN (SELECT id FROM admissions     WHERE resident_id = ?))
			OR (a.entity_type = 'alert_event'    AND a.entity_id IN (SELECT id FROM alert_events   WHERE resident_id = ?))
		)`)
		args = append(args, f.ResidentID, f.ResidentID, f.ResidentID, f.ResidentID)
	}

	q := `
		SELECT a.id, COALESCE(a.user_id,''), COALESCE(u.display_name,'system'),
		       a.action, a.entity_type, a.entity_id, a.details,
		       a.before_snapshot, a.after_snapshot, a.outcome,
		       a.ip_address, a.request_id, a.occurred_at
		FROM audit_logs a
		LEFT JOIN users u ON u.id = a.user_id`

	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY a.occurred_at DESC LIMIT ?"
	args = append(args, f.Limit)

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("audit_repo.ListFiltered: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		var occurredAt string
		if err := rows.Scan(
			&l.ID, &l.UserID, &l.UserName,
			&l.Action, &l.EntityType, &l.EntityID, &l.Details,
			&l.BeforeSnapshot, &l.AfterSnapshot, &l.Outcome,
			&l.IPAddress, &l.RequestID, &occurredAt,
		); err != nil {
			return nil, err
		}
		l.OccurredAt, _ = time.Parse(time.RFC3339, occurredAt)
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// Count returns the total number of audit log entries.
func (r *AuditRepo) Count() (int, error) {
	var n int
	return n, r.db.QueryRow(`SELECT COUNT(*) FROM audit_logs`).Scan(&n)
}

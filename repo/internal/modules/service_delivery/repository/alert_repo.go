package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/service_delivery/domain"
	"careops/clinic/internal/shared/idgen"
)

type AlertRepo struct{ db *sql.DB }

func NewAlertRepo(db *sql.DB) *AlertRepo { return &AlertRepo{db: db} }

// ListUnresolved returns all unresolved alerts, newest first.
func (r *AlertRepo) ListUnresolved() ([]domain.Alert, error) {
	return r.query("WHERE ae.is_resolved = 0 ORDER BY ae.triggered_at DESC", nil)
}

// List returns alerts, optionally filtered by resident and resolved state.
func (r *AlertRepo) List(residentID string, includeResolved bool) ([]domain.Alert, error) {
	where := "WHERE 1=1"
	var args []any
	if !includeResolved {
		where += " AND ae.is_resolved = 0"
	}
	if residentID != "" {
		where += " AND ae.resident_id = ?"
		args = append(args, residentID)
	}
	where += " ORDER BY ae.triggered_at DESC"
	return r.query(where, args)
}

// FindByID returns a single alert or nil if not found.
func (r *AlertRepo) FindByID(id string) (*domain.Alert, error) {
	alerts, err := r.query("WHERE ae.id = ?", []any{id})
	if err != nil {
		return nil, err
	}
	if len(alerts) == 0 {
		return nil, nil
	}
	return &alerts[0], nil
}

// Create inserts a new alert and returns its generated ID.
func (r *AlertRepo) Create(a domain.NewAlert) (string, error) {
	id := idgen.New()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO alert_events
		    (id, resident_id, alert_type, severity, message, is_resolved,
		     triggered_at, created_at)
		VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		id, a.ResidentID, a.AlertType, a.Severity, a.Message, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("alert_repo.Create: %w", err)
	}
	return id, nil
}

// Counts returns (total alerts, unresolved alerts).
func (r *AlertRepo) Counts() (total, unresolved int, err error) {
	err = r.db.QueryRow(`SELECT COUNT(*) FROM alert_events`).Scan(&total)
	if err != nil {
		return
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM alert_events WHERE is_resolved = 0`).Scan(&unresolved)
	return
}

func (r *AlertRepo) query(whereClause string, args []any) ([]domain.Alert, error) {
	q := `
		SELECT ae.id, COALESCE(ae.resident_id,''), ae.alert_type, ae.severity,
		       ae.message, ae.is_resolved, ae.triggered_at,
		       COALESCE(res.first_name || ' ' || res.last_name, 'System')
		FROM alert_events ae
		LEFT JOIN residents res ON res.id = ae.resident_id ` + whereClause

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("alert_repo.query: %w", err)
	}
	defer rows.Close()

	var alerts []domain.Alert
	for rows.Next() {
		var a domain.Alert
		var triggeredAt string
		var isResolved int
		if err := rows.Scan(
			&a.ID, &a.ResidentID, &a.AlertType, &a.Severity,
			&a.Message, &isResolved, &triggeredAt, &a.ResidentName,
		); err != nil {
			return nil, fmt.Errorf("alert_repo scan: %w", err)
		}
		a.IsResolved = isResolved == 1
		a.TriggeredAt, _ = time.Parse(time.RFC3339, triggeredAt)
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

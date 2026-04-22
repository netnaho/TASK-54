package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/finance/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// ShiftRepo manages settlement_shifts rows.
type ShiftRepo struct{ db *sql.DB }

// NewShiftRepo constructs a ShiftRepo.
func NewShiftRepo(db *sql.DB) *ShiftRepo { return &ShiftRepo{db: db} }

// List returns shifts ordered by date desc, newest first.
func (r *ShiftRepo) List(limit int) ([]domain.SettlementShift, error) {
	q := `
		SELECT s.id, s.shift_date, s.shift_name,
		       s.opened_by, COALESCE(s.closed_by,''),
		       s.opened_at, s.closed_at,
		       s.status, s.notes,
		       s.total_amount_cents, s.reconciliation_notes,
		       s.created_at,
		       u.display_name,
		       COALESCE(cu.display_name,'') AS closed_by_name,
		       (SELECT COUNT(*) FROM payments p WHERE p.shift_id = s.id) AS total_payments
		FROM settlement_shifts s
		JOIN users u ON u.id = s.opened_by
		LEFT JOIN users cu ON cu.id = s.closed_by
		ORDER BY s.shift_date DESC, s.shift_name DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("shift_repo.List: %w", err)
	}
	defer rows.Close()
	return scanShiftRows(rows)
}

// FindByID returns the shift or sql.ErrNoRows.
func (r *ShiftRepo) FindByID(id string) (*domain.SettlementShift, error) {
	row := r.db.QueryRow(`
		SELECT s.id, s.shift_date, s.shift_name,
		       s.opened_by, COALESCE(s.closed_by,''),
		       s.opened_at, s.closed_at,
		       s.status, s.notes,
		       s.total_amount_cents, s.reconciliation_notes,
		       s.created_at,
		       u.display_name,
		       COALESCE(cu.display_name,'') AS closed_by_name,
		       (SELECT COUNT(*) FROM payments p WHERE p.shift_id = s.id)
		FROM settlement_shifts s
		JOIN users u ON u.id = s.opened_by
		LEFT JOIN users cu ON cu.id = s.closed_by
		WHERE s.id = ?`, id)
	return scanShiftRow(row)
}

// FindOpen returns the open shift for the given date+name, or nil if none.
func (r *ShiftRepo) FindOpen(date, shiftName string) (*domain.SettlementShift, error) {
	row := r.db.QueryRow(`
		SELECT s.id, s.shift_date, s.shift_name,
		       s.opened_by, COALESCE(s.closed_by,''),
		       s.opened_at, s.closed_at,
		       s.status, s.notes,
		       s.total_amount_cents, s.reconciliation_notes,
		       s.created_at,
		       u.display_name,
		       COALESCE(cu.display_name,'') AS closed_by_name,
		       (SELECT COUNT(*) FROM payments p WHERE p.shift_id = s.id)
		FROM settlement_shifts s
		JOIN users u ON u.id = s.opened_by
		LEFT JOIN users cu ON cu.id = s.closed_by
		WHERE s.shift_date = ? AND s.shift_name = ? AND s.status = 'open'
		LIMIT 1`, date, shiftName)
	s, err := scanShiftRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// Create opens a new shift.
func (r *ShiftRepo) Create(date, shiftName, openedBy string) (*domain.SettlementShift, error) {
	id := idgen.New()
	now := timeutil.NowISO()
	_, err := r.db.Exec(`
		INSERT INTO settlement_shifts
		    (id, shift_date, shift_name, opened_by, opened_at, status,
		     notes, total_amount_cents, reconciliation_notes, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, date, shiftName, openedBy, now, "open",
		"", 0, "", now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("shift_repo.Create: %w", err)
	}
	return r.FindByID(id)
}

// Close finalizes a shift, caching the total and writing reconciliation notes.
func (r *ShiftRepo) Close(id, closedBy string, totalCents int, notes string) error {
	now := timeutil.NowISO()
	res, err := r.db.Exec(`
		UPDATE settlement_shifts
		SET closed_by = ?, closed_at = ?, status = 'closed',
		    total_amount_cents = ?, reconciliation_notes = ?,
		    updated_at = ?
		WHERE id = ? AND status = 'open'`,
		closedBy, now, totalCents, notes, now, id,
	)
	if err != nil {
		return fmt.Errorf("shift_repo.Close: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("shift %s not found or already closed", id)
	}
	return nil
}

// SumPayments returns the total amount_cents for all completed payments in
// the shift, used to compute the cached total at close time.
func (r *ShiftRepo) SumPayments(shiftID string) (int, error) {
	var total sql.NullInt64
	err := r.db.QueryRow(`
		SELECT COALESCE(SUM(amount_cents),0)
		FROM payments
		WHERE shift_id = ? AND status IN ('completed','pending')`, shiftID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("shift_repo.SumPayments: %w", err)
	}
	return int(total.Int64), nil
}

// — scan helpers ——————————————————————————————————————————————————————————————

func scanShiftRow(row *sql.Row) (*domain.SettlementShift, error) {
	var s domain.SettlementShift
	var openedAt, createdAt string
	var closedAt sql.NullString
	err := row.Scan(
		&s.ID, &s.ShiftDate, &s.ShiftName,
		&s.OpenedBy, &s.ClosedBy,
		&openedAt, &closedAt,
		&s.Status, &s.Notes,
		&s.TotalAmountCents, &s.ReconciliationNotes,
		&createdAt,
		&s.OpenedByName, &s.ClosedByName,
		&s.TotalPayments,
	)
	if err != nil {
		return nil, err
	}
	s.OpenedAt, _ = time.Parse(time.RFC3339, openedAt)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if closedAt.Valid && closedAt.String != "" {
		t, err2 := time.Parse(time.RFC3339, closedAt.String)
		if err2 == nil {
			s.ClosedAt = &t
		}
	}
	return &s, nil
}

func scanShiftRows(rows *sql.Rows) ([]domain.SettlementShift, error) {
	var items []domain.SettlementShift
	for rows.Next() {
		var s domain.SettlementShift
		var openedAt, createdAt string
		var closedAt sql.NullString
		if err := rows.Scan(
			&s.ID, &s.ShiftDate, &s.ShiftName,
			&s.OpenedBy, &s.ClosedBy,
			&openedAt, &closedAt,
			&s.Status, &s.Notes,
			&s.TotalAmountCents, &s.ReconciliationNotes,
			&createdAt,
			&s.OpenedByName, &s.ClosedByName,
			&s.TotalPayments,
		); err != nil {
			return nil, err
		}
		s.OpenedAt, _ = time.Parse(time.RFC3339, openedAt)
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if closedAt.Valid && closedAt.String != "" {
			t, _ := time.Parse(time.RFC3339, closedAt.String)
			s.ClosedAt = &t
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

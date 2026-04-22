package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"careops/clinic/internal/modules/finance/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// PaymentRepo manages payments rows.
type PaymentRepo struct{ db *sql.DB }

// NewPaymentRepo constructs a PaymentRepo.
func NewPaymentRepo(db *sql.DB) *PaymentRepo { return &PaymentRepo{db: db} }

// PaymentFilter restricts a List query.
type PaymentFilter struct {
	ShiftID    string
	BatchID    string // filter to a specific card-batch import
	ResidentID string
	DateFrom   string // YYYY-MM-DD
	DateTo     string
	Status     string
	Limit      int
}

// List returns payments matching the filter, newest first.
func (r *PaymentRepo) List(f PaymentFilter) ([]domain.Payment, error) {
	var conds []string
	var args []interface{}

	if f.ShiftID != "" {
		conds = append(conds, "p.shift_id = ?")
		args = append(args, f.ShiftID)
	}
	if f.BatchID != "" {
		conds = append(conds, "p.batch_id = ?")
		args = append(args, f.BatchID)
	}
	if f.ResidentID != "" {
		conds = append(conds, "p.resident_id = ?")
		args = append(args, f.ResidentID)
	}
	if f.DateFrom != "" {
		conds = append(conds, "substr(p.recorded_at,1,10) >= ?")
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		conds = append(conds, "substr(p.recorded_at,1,10) <= ?")
		args = append(args, f.DateTo)
	}
	if f.Status != "" {
		conds = append(conds, "p.status = ?")
		args = append(args, f.Status)
	}

	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	q := `
		SELECT p.id, p.resident_id, COALESCE(p.shift_id,''), COALESCE(p.batch_id,''),
		       p.payment_method, p.amount_cents, p.currency,
		       p.reference_number, p.encrypted_reference,
		       p.description, p.recorded_by, p.status, p.row_version,
		       p.recorded_at, p.created_at, p.updated_at,
		       res.first_name || ' ' || res.last_name AS resident_name,
		       u.display_name AS recorded_by_name,
		       COALESCE((SELECT SUM(rf.amount_cents) FROM refunds rf WHERE rf.payment_id = p.id),0)
		FROM payments p
		JOIN residents res ON res.id = p.resident_id
		JOIN users u ON u.id = p.recorded_by` +
		where + ` ORDER BY p.recorded_at DESC`

	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("payment_repo.List: %w", err)
	}
	defer rows.Close()
	return scanPaymentRows(rows)
}

// FindByID returns the payment or sql.ErrNoRows.
func (r *PaymentRepo) FindByID(id string) (*domain.Payment, error) {
	row := r.db.QueryRow(`
		SELECT p.id, p.resident_id, COALESCE(p.shift_id,''), COALESCE(p.batch_id,''),
		       p.payment_method, p.amount_cents, p.currency,
		       p.reference_number, p.encrypted_reference,
		       p.description, p.recorded_by, p.status, p.row_version,
		       p.recorded_at, p.created_at, p.updated_at,
		       res.first_name || ' ' || res.last_name,
		       u.display_name,
		       COALESCE((SELECT SUM(rf.amount_cents) FROM refunds rf WHERE rf.payment_id = p.id),0)
		FROM payments p
		JOIN residents res ON res.id = p.resident_id
		JOIN users u ON u.id = p.recorded_by
		WHERE p.id = ?`, id)
	return scanPaymentRow(row)
}

// Create inserts a new payment and returns it.
func (r *PaymentRepo) Create(
	residentID, shiftID, batchID, method string,
	amountCents int, currency, displayRef, encRef, desc, recordedBy string,
) (*domain.Payment, error) {
	id := idgen.New()
	now := timeutil.NowISO()
	if currency == "" {
		currency = "USD"
	}
	var shiftArg, batchArg interface{}
	if shiftID != "" {
		shiftArg = shiftID
	}
	if batchID != "" {
		batchArg = batchID
	}
	_, err := r.db.Exec(`
		INSERT INTO payments
		    (id, resident_id, shift_id, batch_id, payment_method,
		     amount_cents, currency, reference_number, encrypted_reference,
		     description, recorded_by, status, row_version,
		     recorded_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		id, residentID, shiftArg, batchArg, method,
		amountCents, currency, displayRef, encRef,
		desc, recordedBy, "completed", 1,
		now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("payment_repo.Create: %w", err)
	}
	return r.FindByID(id)
}

// DB exposes the underlying connection for cross-table queries in the service.
func (r *PaymentRepo) DB() *sql.DB { return r.db }

// UpdateStatus changes the status of a payment using optimistic locking.
func (r *PaymentRepo) UpdateStatus(id, status string, rowVersion int) error {
	now := timeutil.NowISO()
	res, err := r.db.Exec(`
		UPDATE payments SET status = ?, row_version = row_version + 1, updated_at = ?
		WHERE id = ? AND row_version = ?`, status, now, id, rowVersion)
	if err != nil {
		return fmt.Errorf("payment_repo.UpdateStatus: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("version conflict updating payment %s", id)
	}
	return nil
}

// — scan helpers ——————————————————————————————————————————————————————————————

func scanPaymentRow(row *sql.Row) (*domain.Payment, error) {
	var p domain.Payment
	var recAt, creAt, updAt string
	err := row.Scan(
		&p.ID, &p.ResidentID, &p.ShiftID, &p.BatchID,
		&p.PaymentMethod, &p.AmountCents, &p.Currency,
		&p.ReferenceNumber, &p.EncryptedReference,
		&p.Description, &p.RecordedBy, &p.Status, &p.RowVersion,
		&recAt, &creAt, &updAt,
		&p.ResidentName, &p.RecordedByName,
		&p.TotalRefunded,
	)
	if err != nil {
		return nil, err
	}
	p.RecordedAt, _ = time.Parse(time.RFC3339, recAt)
	p.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
	p.UpdatedAt, _ = time.Parse(time.RFC3339, updAt)
	return &p, nil
}

func scanPaymentRows(rows *sql.Rows) ([]domain.Payment, error) {
	var items []domain.Payment
	for rows.Next() {
		var p domain.Payment
		var recAt, creAt, updAt string
		if err := rows.Scan(
			&p.ID, &p.ResidentID, &p.ShiftID, &p.BatchID,
			&p.PaymentMethod, &p.AmountCents, &p.Currency,
			&p.ReferenceNumber, &p.EncryptedReference,
			&p.Description, &p.RecordedBy, &p.Status, &p.RowVersion,
			&recAt, &creAt, &updAt,
			&p.ResidentName, &p.RecordedByName,
			&p.TotalRefunded,
		); err != nil {
			return nil, err
		}
		p.RecordedAt, _ = time.Parse(time.RFC3339, recAt)
		p.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
		p.UpdatedAt, _ = time.Parse(time.RFC3339, updAt)
		items = append(items, p)
	}
	return items, rows.Err()
}

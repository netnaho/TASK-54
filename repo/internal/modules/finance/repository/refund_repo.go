package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/finance/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// RefundRepo manages refunds rows.
type RefundRepo struct{ db *sql.DB }

// NewRefundRepo constructs a RefundRepo.
func NewRefundRepo(db *sql.DB) *RefundRepo { return &RefundRepo{db: db} }

// Create inserts a new refund and returns it.
func (r *RefundRepo) Create(paymentID, processedBy string, amountCents int, reason string) (*domain.Refund, error) {
	id := idgen.New()
	now := timeutil.NowISO()
	_, err := r.db.Exec(`
		INSERT INTO refunds (id, payment_id, amount_cents, reason, processed_by, processed_at, created_at)
		VALUES (?,?,?,?,?,?,?)`,
		id, paymentID, amountCents, reason, processedBy, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("refund_repo.Create: %w", err)
	}
	return r.FindByID(id)
}

// FindByID returns the refund or sql.ErrNoRows.
func (r *RefundRepo) FindByID(id string) (*domain.Refund, error) {
	row := r.db.QueryRow(`
		SELECT rf.id, rf.payment_id, rf.amount_cents, rf.reason,
		       rf.processed_by, rf.processed_at, rf.created_at,
		       u.display_name,
		       res.first_name || ' ' || res.last_name
		FROM refunds rf
		JOIN users u ON u.id = rf.processed_by
		JOIN payments p ON p.id = rf.payment_id
		JOIN residents res ON res.id = p.resident_id
		WHERE rf.id = ?`, id)
	return scanRefundRow(row)
}

// ListByPayment returns all refunds for a payment.
func (r *RefundRepo) ListByPayment(paymentID string) ([]domain.Refund, error) {
	rows, err := r.db.Query(`
		SELECT rf.id, rf.payment_id, rf.amount_cents, rf.reason,
		       rf.processed_by, rf.processed_at, rf.created_at,
		       u.display_name,
		       res.first_name || ' ' || res.last_name
		FROM refunds rf
		JOIN users u ON u.id = rf.processed_by
		JOIN payments p ON p.id = rf.payment_id
		JOIN residents res ON res.id = p.resident_id
		WHERE rf.payment_id = ?
		ORDER BY rf.processed_at DESC`, paymentID)
	if err != nil {
		return nil, fmt.Errorf("refund_repo.ListByPayment: %w", err)
	}
	defer rows.Close()
	var items []domain.Refund
	for rows.Next() {
		var rf domain.Refund
		var procAt, creAt string
		if err := rows.Scan(
			&rf.ID, &rf.PaymentID, &rf.AmountCents, &rf.Reason,
			&rf.ProcessedBy, &procAt, &creAt,
			&rf.ProcessedByName, &rf.ResidentName,
		); err != nil {
			return nil, err
		}
		rf.ProcessedAt, _ = time.Parse(time.RFC3339, procAt)
		rf.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
		items = append(items, rf)
	}
	return items, rows.Err()
}

// TotalRefundedForPayment returns the sum of all refund amounts for a payment.
func (r *RefundRepo) TotalRefundedForPayment(paymentID string) (int, error) {
	var total sql.NullInt64
	err := r.db.QueryRow(`SELECT COALESCE(SUM(amount_cents),0) FROM refunds WHERE payment_id=?`, paymentID).Scan(&total)
	return int(total.Int64), err
}

func scanRefundRow(row *sql.Row) (*domain.Refund, error) {
	var rf domain.Refund
	var procAt, creAt string
	if err := row.Scan(
		&rf.ID, &rf.PaymentID, &rf.AmountCents, &rf.Reason,
		&rf.ProcessedBy, &procAt, &creAt,
		&rf.ProcessedByName, &rf.ResidentName,
	); err != nil {
		return nil, err
	}
	rf.ProcessedAt, _ = time.Parse(time.RFC3339, procAt)
	rf.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
	return &rf, nil
}

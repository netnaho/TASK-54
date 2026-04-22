package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/service_delivery/domain"
)

// OrderFilter is the set of optional filters for listing service orders.
type OrderFilter struct {
	ResidentID  string
	Status      string // "active" | "completed" | "cancelled" | "" (all)
	ServiceType string
	DateFrom    string // ISO date, inclusive
	DateTo      string // ISO date, inclusive
}

type ServiceOrderRepo struct{ db *sql.DB }

func NewServiceOrderRepo(db *sql.DB) *ServiceOrderRepo { return &ServiceOrderRepo{db: db} }

// List returns service orders matching the filter, ordered newest first.
func (r *ServiceOrderRepo) List(f OrderFilter) ([]domain.ServiceOrder, error) {
	query := `
		SELECT so.id, so.resident_id, so.ordered_by, so.service_type, so.frequency,
		       so.instructions, so.status, so.start_date, COALESCE(so.end_date,''),
		       so.row_version, so.created_at, so.updated_at,
		       res.first_name || ' ' || res.last_name,
		       u.display_name
		FROM service_orders so
		JOIN residents res ON res.id = so.resident_id
		JOIN users u ON u.id = so.ordered_by
		WHERE 1=1`

	var args []any
	if f.ResidentID != "" {
		query += " AND so.resident_id = ?"
		args = append(args, f.ResidentID)
	}
	if f.Status != "" {
		query += " AND so.status = ?"
		args = append(args, f.Status)
	}
	if f.ServiceType != "" {
		query += " AND so.service_type = ?"
		args = append(args, f.ServiceType)
	}
	if f.DateFrom != "" {
		query += " AND so.start_date >= ?"
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		query += " AND so.start_date <= ?"
		args = append(args, f.DateTo)
	}
	query += " ORDER BY so.created_at DESC"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("order_repo.List: %w", err)
	}
	defer rows.Close()

	var orders []domain.ServiceOrder
	for rows.Next() {
		var o domain.ServiceOrder
		var ca, ua string
		if err := rows.Scan(
			&o.ID, &o.ResidentID, &o.OrderedBy, &o.ServiceType, &o.Frequency,
			&o.Instructions, &o.Status, &o.StartDate, &o.EndDate, &o.RowVersion, &ca, &ua,
			&o.ResidentName, &o.OrderedByName,
		); err != nil {
			return nil, fmt.Errorf("order_repo.List scan: %w", err)
		}
		o.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		o.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// FindByID returns the order with full resident and ordered-by names, or nil if not found.
func (r *ServiceOrderRepo) FindByID(id string) (*domain.ServiceOrder, error) {
	var o domain.ServiceOrder
	var ca, ua string
	err := r.db.QueryRow(`
		SELECT so.id, so.resident_id, so.ordered_by, so.service_type, so.frequency,
		       so.instructions, so.status, so.start_date, COALESCE(so.end_date,''),
		       so.row_version, so.created_at, so.updated_at,
		       res.first_name || ' ' || res.last_name,
		       u.display_name
		FROM service_orders so
		JOIN residents res ON res.id = so.resident_id
		JOIN users u ON u.id = so.ordered_by
		WHERE so.id = ?`, id,
	).Scan(
		&o.ID, &o.ResidentID, &o.OrderedBy, &o.ServiceType, &o.Frequency,
		&o.Instructions, &o.Status, &o.StartDate, &o.EndDate, &o.RowVersion, &ca, &ua,
		&o.ResidentName, &o.OrderedByName,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("order_repo.FindByID: %w", err)
	}
	o.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	o.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &o, nil
}

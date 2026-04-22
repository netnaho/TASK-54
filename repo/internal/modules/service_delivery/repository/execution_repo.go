package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/service_delivery/domain"
)

type ExecutionFilter struct {
	ServiceOrderID string
	PerformedBy    string
	DateFrom       string // ISO datetime, inclusive
	DateTo         string // ISO datetime, inclusive
}

type ExecutionRecordRepo struct{ db *sql.DB }

func NewExecutionRecordRepo(db *sql.DB) *ExecutionRecordRepo {
	return &ExecutionRecordRepo{db: db}
}

// List returns execution records matching the filter, newest first.
func (r *ExecutionRecordRepo) List(f ExecutionFilter) ([]domain.ExecutionRecord, error) {
	query := `
		SELECT ser.id, ser.service_order_id, ser.performed_by,
		       COALESCE(ser.scheduled_start,''), ser.performed_at,
		       ser.duration_minutes, ser.notes, ser.status, ser.created_at,
		       u.display_name,
		       res.first_name || ' ' || res.last_name,
		       so.service_type
		FROM service_execution_records ser
		JOIN users u ON u.id = ser.performed_by
		JOIN service_orders so ON so.id = ser.service_order_id
		JOIN residents res ON res.id = so.resident_id
		WHERE 1=1`

	var args []any
	if f.ServiceOrderID != "" {
		query += " AND ser.service_order_id = ?"
		args = append(args, f.ServiceOrderID)
	}
	if f.PerformedBy != "" {
		query += " AND ser.performed_by = ?"
		args = append(args, f.PerformedBy)
	}
	if f.DateFrom != "" {
		query += " AND ser.performed_at >= ?"
		args = append(args, f.DateFrom)
	}
	if f.DateTo != "" {
		query += " AND ser.performed_at <= ?"
		args = append(args, f.DateTo)
	}
	query += " ORDER BY ser.performed_at DESC LIMIT 50"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("execution_repo.List: %w", err)
	}
	defer rows.Close()

	return scanExecutionRows(rows)
}

// ListByOrderID returns all executions for a specific service order.
func (r *ExecutionRecordRepo) ListByOrderID(orderID string) ([]domain.ExecutionRecord, error) {
	rows, err := r.db.Query(`
		SELECT ser.id, ser.service_order_id, ser.performed_by,
		       COALESCE(ser.scheduled_start,''), ser.performed_at,
		       ser.duration_minutes, ser.notes, ser.status, ser.created_at,
		       u.display_name,
		       res.first_name || ' ' || res.last_name,
		       so.service_type
		FROM service_execution_records ser
		JOIN users u ON u.id = ser.performed_by
		JOIN service_orders so ON so.id = ser.service_order_id
		JOIN residents res ON res.id = so.resident_id
		WHERE ser.service_order_id = ?
		ORDER BY ser.performed_at DESC`, orderID)
	if err != nil {
		return nil, fmt.Errorf("execution_repo.ListByOrderID: %w", err)
	}
	defer rows.Close()
	return scanExecutionRows(rows)
}

// ComputeMetrics calculates execution and timeliness metrics for a date range.
// dateFrom and dateTo are ISO datetime strings; empty string means unbounded.
func (r *ExecutionRecordRepo) ComputeMetrics(dateFrom, dateTo string) (domain.ExecutionMetrics, error) {
	var m domain.ExecutionMetrics

	// Order status counts
	err := r.db.QueryRow(`
		SELECT
		    COUNT(*) AS total,
		    SUM(CASE WHEN status='active'    THEN 1 ELSE 0 END),
		    SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END),
		    SUM(CASE WHEN status='cancelled' THEN 1 ELSE 0 END)
		FROM service_orders`).Scan(&m.TotalOrders, &m.ActiveOrders, &m.CompletedOrders, &m.CancelledOrders)
	if err != nil {
		return m, fmt.Errorf("execution_repo.ComputeMetrics orders: %w", err)
	}

	// Execution status counts + timeliness
	execQuery := `
		SELECT
		    COUNT(*) AS total,
		    SUM(CASE WHEN status='completed' THEN 1 ELSE 0 END),
		    SUM(CASE WHEN status='refused'   THEN 1 ELSE 0 END),
		    SUM(CASE WHEN status='missed'    THEN 1 ELSE 0 END),
		    SUM(CASE WHEN scheduled_start != '' THEN 1 ELSE 0 END),
		    SUM(CASE WHEN scheduled_start != ''
		             AND CAST((julianday(performed_at) - julianday(scheduled_start)) * 1440 AS INTEGER) <= ?
		             THEN 1 ELSE 0 END),
		    SUM(CASE WHEN scheduled_start != ''
		             AND CAST((julianday(performed_at) - julianday(scheduled_start)) * 1440 AS INTEGER) > ?
		             THEN 1 ELSE 0 END)
		FROM service_execution_records
		WHERE 1=1`

	args := []any{
		domain.TimelinessThresholdMinutes,
		domain.TimelinessThresholdMinutes,
	}
	if dateFrom != "" {
		execQuery += " AND performed_at >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		execQuery += " AND performed_at <= ?"
		args = append(args, dateTo)
	}

	err = r.db.QueryRow(execQuery, args...).Scan(
		&m.TotalExecutions, &m.CompletedExecutions, &m.RefusedExecutions, &m.MissedExecutions,
		&m.ScheduledExecutions, &m.OnTimeCount, &m.LateCount,
	)
	if err != nil {
		return m, fmt.Errorf("execution_repo.ComputeMetrics executions: %w", err)
	}

	finished := m.CompletedExecutions + m.RefusedExecutions + m.MissedExecutions
	if finished > 0 {
		m.CompletionRate = float64(m.CompletedExecutions) / float64(finished)
	}
	if m.ScheduledExecutions > 0 {
		m.OnTimeRate = float64(m.OnTimeCount) / float64(m.ScheduledExecutions)
	}

	return m, nil
}

func scanExecutionRows(rows *sql.Rows) ([]domain.ExecutionRecord, error) {
	var records []domain.ExecutionRecord
	for rows.Next() {
		var rec domain.ExecutionRecord
		var scheduledStart, performedAt, ca string
		if err := rows.Scan(
			&rec.ID, &rec.ServiceOrderID, &rec.PerformedBy,
			&scheduledStart, &performedAt,
			&rec.DurationMinutes, &rec.Notes, &rec.Status, &ca,
			&rec.PerformedByName, &rec.ResidentName, &rec.ServiceType,
		); err != nil {
			return nil, fmt.Errorf("execution_repo scan: %w", err)
		}
		if scheduledStart != "" {
			rec.ScheduledStart, _ = time.Parse(time.RFC3339, scheduledStart)
		}
		rec.PerformedAt, _ = time.Parse(time.RFC3339, performedAt)
		rec.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		records = append(records, rec)
	}
	return records, rows.Err()
}

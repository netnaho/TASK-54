package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/service_delivery/domain"
	"careops/clinic/internal/shared/idgen"
)

type CareCheckpointRepo struct{ db *sql.DB }

func NewCareCheckpointRepo(db *sql.DB) *CareCheckpointRepo {
	return &CareCheckpointRepo{db: db}
}

// List returns checkpoints, optionally filtered by resident and date range.
func (r *CareCheckpointRepo) List(residentID, dateFrom, dateTo string) ([]domain.CareCheckpoint, error) {
	query := `
		SELECT cqc.id, cqc.resident_id, cqc.checked_by, cqc.checkpoint_type,
		       cqc.score, cqc.notes, cqc.checked_at, cqc.created_at,
		       res.first_name || ' ' || res.last_name,
		       u.display_name
		FROM care_quality_checkpoints cqc
		JOIN residents res ON res.id = cqc.resident_id
		JOIN users u ON u.id = cqc.checked_by
		WHERE 1=1`

	var args []any
	if residentID != "" {
		query += " AND cqc.resident_id = ?"
		args = append(args, residentID)
	}
	if dateFrom != "" {
		query += " AND cqc.checked_at >= ?"
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		query += " AND cqc.checked_at <= ?"
		args = append(args, dateTo)
	}
	query += " ORDER BY cqc.checked_at DESC LIMIT 50"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("checkpoint_repo.List: %w", err)
	}
	defer rows.Close()

	var items []domain.CareCheckpoint
	for rows.Next() {
		var cp domain.CareCheckpoint
		var checkedAt, ca string
		if err := rows.Scan(
			&cp.ID, &cp.ResidentID, &cp.CheckedBy, &cp.CheckpointType,
			&cp.Score, &cp.Notes, &checkedAt, &ca,
			&cp.ResidentName, &cp.CheckedByName,
		); err != nil {
			return nil, fmt.Errorf("checkpoint_repo scan: %w", err)
		}
		cp.CheckedAt, _ = time.Parse(time.RFC3339, checkedAt)
		cp.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		items = append(items, cp)
	}
	return items, rows.Err()
}

// Create inserts a new care quality checkpoint and returns its generated ID.
func (r *CareCheckpointRepo) Create(cp domain.NewCareCheckpoint) (string, error) {
	id := idgen.New()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO care_quality_checkpoints
		    (id, resident_id, checked_by, checkpoint_type, score, notes, checked_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, cp.ResidentID, cp.CheckedBy, cp.CheckpointType, cp.Score, cp.Notes, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("checkpoint_repo.Create: %w", err)
	}
	return id, nil
}

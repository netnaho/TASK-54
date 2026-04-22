package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/finance/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// BatchRepo manages imported_card_batches rows.
type BatchRepo struct{ db *sql.DB }

// NewBatchRepo constructs a BatchRepo.
func NewBatchRepo(db *sql.DB) *BatchRepo { return &BatchRepo{db: db} }

// Create inserts a new batch record (initially 'processing') and returns it.
func (r *BatchRepo) Create(fileName, filePath, importedBy, shiftID string) (*domain.CardBatch, error) {
	id := idgen.New()
	now := timeutil.NowISO()
	var shiftArg interface{}
	if shiftID != "" {
		shiftArg = shiftID
	}
	_, err := r.db.Exec(`
		INSERT INTO imported_card_batches
		    (id, file_name, file_path, imported_by, shift_id,
		     record_count, success_count, error_count,
		     status, error_detail, imported_at, created_at)
		VALUES (?,?,?,?,?,0,0,0,'processing','',?,?)`,
		id, fileName, filePath, importedBy, shiftArg, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("batch_repo.Create: %w", err)
	}
	return r.FindByID(id)
}

// FindByID returns the batch or sql.ErrNoRows.
func (r *BatchRepo) FindByID(id string) (*domain.CardBatch, error) {
	row := r.db.QueryRow(`
		SELECT b.id, b.file_name, b.file_path,
		       COALESCE(b.shift_id,''),
		       b.imported_by,
		       b.record_count, b.success_count, b.error_count,
		       b.status, b.error_detail,
		       b.imported_at, b.created_at,
		       u.display_name,
		       COALESCE(s.shift_date,'')
		FROM imported_card_batches b
		JOIN users u ON u.id = b.imported_by
		LEFT JOIN settlement_shifts s ON s.id = b.shift_id
		WHERE b.id = ?`, id)
	return scanBatchRow(row)
}

// List returns the last N batches, newest first.
func (r *BatchRepo) List(limit int) ([]domain.CardBatch, error) {
	q := `
		SELECT b.id, b.file_name, b.file_path,
		       COALESCE(b.shift_id,''),
		       b.imported_by,
		       b.record_count, b.success_count, b.error_count,
		       b.status, b.error_detail,
		       b.imported_at, b.created_at,
		       u.display_name,
		       COALESCE(s.shift_date,'')
		FROM imported_card_batches b
		JOIN users u ON u.id = b.imported_by
		LEFT JOIN settlement_shifts s ON s.id = b.shift_id
		ORDER BY b.imported_at DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("batch_repo.List: %w", err)
	}
	defer rows.Close()
	var items []domain.CardBatch
	for rows.Next() {
		var b domain.CardBatch
		var impAt, creAt string
		if err := rows.Scan(
			&b.ID, &b.FileName, &b.FilePath,
			&b.ShiftID, &b.ImportedBy,
			&b.RecordCount, &b.SuccessCount, &b.ErrorCount,
			&b.Status, &b.ErrorDetail,
			&impAt, &creAt,
			&b.ImportedByName, &b.ShiftDate,
		); err != nil {
			return nil, err
		}
		b.ImportedAt, _ = time.Parse(time.RFC3339, impAt)
		b.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
		items = append(items, b)
	}
	return items, rows.Err()
}

// SetResult records the final status and counts.
func (r *BatchRepo) SetResult(id string, recordCount, successCount, errorCount int, status, errDetail string) error {
	_, err := r.db.Exec(`
		UPDATE imported_card_batches
		SET record_count=?, success_count=?, error_count=?, status=?, error_detail=?
		WHERE id=?`,
		recordCount, successCount, errorCount, status, errDetail, id,
	)
	if err != nil {
		return fmt.Errorf("batch_repo.SetResult: %w", err)
	}
	return nil
}

func scanBatchRow(row *sql.Row) (*domain.CardBatch, error) {
	var b domain.CardBatch
	var impAt, creAt string
	if err := row.Scan(
		&b.ID, &b.FileName, &b.FilePath,
		&b.ShiftID, &b.ImportedBy,
		&b.RecordCount, &b.SuccessCount, &b.ErrorCount,
		&b.Status, &b.ErrorDetail,
		&impAt, &creAt,
		&b.ImportedByName, &b.ShiftDate,
	); err != nil {
		return nil, err
	}
	b.ImportedAt, _ = time.Parse(time.RFC3339, impAt)
	b.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
	return &b, nil
}

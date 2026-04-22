package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/beds/domain"
)

type BedRepo struct {
	db *sql.DB
}

func NewBedRepo(db *sql.DB) *BedRepo {
	return &BedRepo{db: db}
}

// ListWithOccupancy returns all active beds with current occupancy status.
func (r *BedRepo) ListWithOccupancy() ([]domain.Bed, error) {
	rows, err := r.db.Query(`
		SELECT b.id, b.ward, b.room_number, b.bed_label, b.bed_type, b.is_active,
		       b.row_version, b.created_at, b.updated_at,
		       COALESCE(r.first_name || ' ' || r.last_name, '') AS resident_name,
		       CASE WHEN a.id IS NOT NULL THEN 1 ELSE 0 END AS is_occupied
		FROM beds b
		LEFT JOIN admissions a ON a.bed_id = b.id AND a.discharged_at IS NULL
		LEFT JOIN residents r  ON r.id = a.resident_id
		WHERE b.is_active = 1
		ORDER BY b.ward, b.room_number, b.bed_label`)
	if err != nil {
		return nil, fmt.Errorf("bed_repo.ListWithOccupancy: %w", err)
	}
	defer rows.Close()

	var beds []domain.Bed
	for rows.Next() {
		var b domain.Bed
		var ca, ua string
		var isActive, isOccupied int
		if err := rows.Scan(
			&b.ID, &b.Ward, &b.RoomNumber, &b.BedLabel, &b.BedType, &isActive,
			&b.RowVersion, &ca, &ua,
			&b.CurrentResidentName, &isOccupied,
		); err != nil {
			return nil, fmt.Errorf("bed_repo scan: %w", err)
		}
		b.IsActive = isActive == 1
		b.IsOccupied = isOccupied == 1
		b.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		beds = append(beds, b)
	}
	return beds, rows.Err()
}

// FindByID returns the bed with occupancy detail, or nil if not found.
func (r *BedRepo) FindByID(id string) (*domain.Bed, error) {
	var b domain.Bed
	var ca, ua string
	var isActive, isOccupied int
	err := r.db.QueryRow(`
		SELECT b.id, b.ward, b.room_number, b.bed_label, b.bed_type, b.is_active,
		       b.row_version, b.created_at, b.updated_at,
		       COALESCE(r.first_name || ' ' || r.last_name, '') AS resident_name,
		       CASE WHEN a.id IS NOT NULL THEN 1 ELSE 0 END AS is_occupied
		FROM beds b
		LEFT JOIN admissions a ON a.bed_id = b.id AND a.discharged_at IS NULL
		LEFT JOIN residents r  ON r.id = a.resident_id
		WHERE b.id = ?`, id,
	).Scan(&b.ID, &b.Ward, &b.RoomNumber, &b.BedLabel, &b.BedType, &isActive,
		&b.RowVersion, &ca, &ua, &b.CurrentResidentName, &isOccupied)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("bed_repo.FindByID: %w", err)
	}
	b.IsActive = isActive == 1
	b.IsOccupied = isOccupied == 1
	b.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	b.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &b, nil
}

// ListByWard returns beds filtered by ward name (case-sensitive).
func (r *BedRepo) ListByWard(ward string) ([]domain.Bed, error) {
	rows, err := r.db.Query(`
		SELECT b.id, b.ward, b.room_number, b.bed_label, b.bed_type, b.is_active,
		       b.row_version, b.created_at, b.updated_at,
		       COALESCE(r.first_name || ' ' || r.last_name, '') AS resident_name,
		       CASE WHEN a.id IS NOT NULL THEN 1 ELSE 0 END AS is_occupied
		FROM beds b
		LEFT JOIN admissions a ON a.bed_id = b.id AND a.discharged_at IS NULL
		LEFT JOIN residents r  ON r.id = a.resident_id
		WHERE b.is_active = 1 AND b.ward = ?
		ORDER BY b.room_number, b.bed_label`, ward)
	if err != nil {
		return nil, fmt.Errorf("bed_repo.ListByWard: %w", err)
	}
	defer rows.Close()
	return scanBedRows(rows)
}

// DistinctWards returns the unique ward names for active beds.
func (r *BedRepo) DistinctWards() ([]string, error) {
	rows, err := r.db.Query(`SELECT DISTINCT ward FROM beds WHERE is_active = 1 ORDER BY ward`)
	if err != nil {
		return nil, fmt.Errorf("bed_repo.DistinctWards: %w", err)
	}
	defer rows.Close()
	var wards []string
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		wards = append(wards, w)
	}
	return wards, rows.Err()
}

// Counts returns total and occupied bed counts.
func (r *BedRepo) Counts() (total, occupied int, err error) {
	err = r.db.QueryRow(`SELECT COUNT(*) FROM beds WHERE is_active = 1`).Scan(&total)
	if err != nil {
		return
	}
	err = r.db.QueryRow(`
		SELECT COUNT(*) FROM admissions WHERE discharged_at IS NULL`).Scan(&occupied)
	return
}

func scanBedRows(rows *sql.Rows) ([]domain.Bed, error) {
	var beds []domain.Bed
	for rows.Next() {
		var b domain.Bed
		var ca, ua string
		var isActive, isOccupied int
		if err := rows.Scan(
			&b.ID, &b.Ward, &b.RoomNumber, &b.BedLabel, &b.BedType, &isActive,
			&b.RowVersion, &ca, &ua,
			&b.CurrentResidentName, &isOccupied,
		); err != nil {
			return nil, fmt.Errorf("bed_repo scan: %w", err)
		}
		b.IsActive = isActive == 1
		b.IsOccupied = isOccupied == 1
		b.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		beds = append(beds, b)
	}
	return beds, rows.Err()
}

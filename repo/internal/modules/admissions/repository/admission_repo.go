package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/admissions/domain"
)

type AdmissionRepo struct {
	db *sql.DB
}

func NewAdmissionRepo(db *sql.DB) *AdmissionRepo {
	return &AdmissionRepo{db: db}
}

// ListResidentsWithStatus returns all residents with their current admission info.
func (r *AdmissionRepo) ListResidentsWithStatus() ([]domain.Resident, error) {
	rows, err := r.db.Query(`
		SELECT res.id, res.mrn, res.first_name, res.last_name, res.date_of_birth,
		       res.gender, res.primary_diagnosis, res.care_level, res.row_version,
		       res.created_at, res.updated_at,
		       COALESCE(b.ward || ' / ' || b.room_number || '-' || b.bed_label, '') AS bed_location,
		       COALESCE(a.admitted_at, '') AS admission_date,
		       CASE WHEN a.id IS NOT NULL THEN 1 ELSE 0 END AS is_admitted
		FROM residents res
		LEFT JOIN admissions a ON a.resident_id = res.id AND a.discharged_at IS NULL
		LEFT JOIN beds b ON b.id = a.bed_id
		ORDER BY res.last_name, res.first_name`)
	if err != nil {
		return nil, fmt.Errorf("admission_repo.ListResidents: %w", err)
	}
	defer rows.Close()

	var residents []domain.Resident
	for rows.Next() {
		var res domain.Resident
		var ca, ua string
		var isAdmitted int
		if err := rows.Scan(
			&res.ID, &res.MRN, &res.FirstName, &res.LastName, &res.DateOfBirth,
			&res.Gender, &res.PrimaryDiagnosis, &res.CareLevel, &res.RowVersion,
			&ca, &ua, &res.CurrentBed, &res.AdmissionDate, &isAdmitted,
		); err != nil {
			return nil, fmt.Errorf("admission_repo scan: %w", err)
		}
		res.IsCurrentlyAdmitted = isAdmitted == 1
		res.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		res.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		residents = append(residents, res)
	}
	return residents, rows.Err()
}

// FindResident returns a single resident with current admission status, or nil.
func (r *AdmissionRepo) FindResident(id string) (*domain.Resident, error) {
	var res domain.Resident
	var ca, ua string
	var isAdmitted int
	err := r.db.QueryRow(`
		SELECT res.id, res.mrn, res.first_name, res.last_name, res.date_of_birth,
		       res.gender, res.primary_diagnosis, res.care_level, res.row_version,
		       res.created_at, res.updated_at,
		       COALESCE(b.ward || ' / ' || b.room_number || '-' || b.bed_label, '') AS bed_location,
		       COALESCE(a.admitted_at, '') AS admission_date,
		       CASE WHEN a.id IS NOT NULL THEN 1 ELSE 0 END AS is_admitted
		FROM residents res
		LEFT JOIN admissions a ON a.resident_id = res.id AND a.discharged_at IS NULL
		LEFT JOIN beds b ON b.id = a.bed_id
		WHERE res.id = ?`, id,
	).Scan(
		&res.ID, &res.MRN, &res.FirstName, &res.LastName, &res.DateOfBirth,
		&res.Gender, &res.PrimaryDiagnosis, &res.CareLevel, &res.RowVersion,
		&ca, &ua, &res.CurrentBed, &res.AdmissionDate, &isAdmitted,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("admission_repo.FindResident: %w", err)
	}
	res.IsCurrentlyAdmitted = isAdmitted == 1
	res.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	res.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &res, nil
}

// ListActiveResidents returns id+name pairs for residents with active admissions (for form selects).
func (r *AdmissionRepo) ListActiveResidents() ([]domain.ResidentOption, error) {
	rows, err := r.db.Query(`
		SELECT res.id, res.first_name || ' ' || res.last_name, res.mrn
		FROM residents res
		JOIN admissions a ON a.resident_id = res.id AND a.discharged_at IS NULL
		ORDER BY res.last_name, res.first_name`)
	if err != nil {
		return nil, fmt.Errorf("admission_repo.ListActiveResidents: %w", err)
	}
	defer rows.Close()
	var opts []domain.ResidentOption
	for rows.Next() {
		var o domain.ResidentOption
		if err := rows.Scan(&o.ID, &o.FullName, &o.MRN); err != nil {
			return nil, err
		}
		opts = append(opts, o)
	}
	return opts, rows.Err()
}

// Counts returns total residents and active admissions.
func (r *AdmissionRepo) Counts() (totalResidents, activeAdmissions int, err error) {
	if err = r.db.QueryRow(`SELECT COUNT(*) FROM residents`).Scan(&totalResidents); err != nil {
		return
	}
	err = r.db.QueryRow(`SELECT COUNT(*) FROM admissions WHERE discharged_at IS NULL`).Scan(&activeAdmissions)
	return
}

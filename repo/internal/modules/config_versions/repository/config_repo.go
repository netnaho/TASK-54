package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/config_versions/domain"
	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// ConfigRepo manages config_versions rows.
type ConfigRepo struct{ db *sql.DB }

// NewConfigRepo constructs a ConfigRepo.
func NewConfigRepo(db *sql.DB) *ConfigRepo { return &ConfigRepo{db: db} }

// List returns all config entries ordered by namespace then key.
func (r *ConfigRepo) List() ([]domain.ConfigVersion, error) {
	rows, err := r.db.Query(`
		SELECT cv.id, cv.namespace, cv.config_key, cv.config_value,
		       cv.description, cv.changed_by, cv.row_version,
		       cv.created_at, cv.updated_at,
		       u.display_name
		FROM config_versions cv
		JOIN users u ON u.id = cv.changed_by
		ORDER BY cv.namespace, cv.config_key`)
	if err != nil {
		return nil, fmt.Errorf("config_repo.List: %w", err)
	}
	defer rows.Close()
	return scanRows(rows)
}

// FindByID returns a config entry or sql.ErrNoRows.
func (r *ConfigRepo) FindByID(id string) (*domain.ConfigVersion, error) {
	row := r.db.QueryRow(`
		SELECT cv.id, cv.namespace, cv.config_key, cv.config_value,
		       cv.description, cv.changed_by, cv.row_version,
		       cv.created_at, cv.updated_at,
		       u.display_name
		FROM config_versions cv
		JOIN users u ON u.id = cv.changed_by
		WHERE cv.id = ?`, id)
	return scanRow(row)
}

// FindByKey looks up a config entry by namespace + key.
func (r *ConfigRepo) FindByKey(namespace, key string) (*domain.ConfigVersion, error) {
	row := r.db.QueryRow(`
		SELECT cv.id, cv.namespace, cv.config_key, cv.config_value,
		       cv.description, cv.changed_by, cv.row_version,
		       cv.created_at, cv.updated_at,
		       u.display_name
		FROM config_versions cv
		JOIN users u ON u.id = cv.changed_by
		WHERE cv.namespace = ? AND cv.config_key = ?`, namespace, key)
	cv, err := scanRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return cv, err
}

// UpdateValue updates the value using optimistic locking.
func (r *ConfigRepo) UpdateValue(id, newValue, changedBy string, rowVersion int) error {
	now := timeutil.NowISO()
	res, err := r.db.Exec(`
		UPDATE config_versions
		SET config_value = ?, changed_by = ?, row_version = row_version + 1, updated_at = ?
		WHERE id = ? AND row_version = ?`,
		newValue, changedBy, now, id, rowVersion,
	)
	if err != nil {
		return fmt.Errorf("config_repo.UpdateValue: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return &apperr.ConflictError{Entity: "config_versions", ID: id}
	}
	return nil
}

// Upsert creates or updates a config entry (used during rollback).
func (r *ConfigRepo) Upsert(namespace, key, value, changedBy string) error {
	now := timeutil.NowISO()

	existing, err := r.FindByKey(namespace, key)
	if err != nil {
		return err
	}
	if existing == nil {
		id := idgen.New()
		_, err = r.db.Exec(`
			INSERT INTO config_versions
			    (id, namespace, config_key, config_value, description, changed_by,
			     row_version, created_at, updated_at)
			VALUES (?,?,?,?,?,?,1,?,?)`,
			id, namespace, key, value, "", changedBy, now, now,
		)
	} else {
		_, err = r.db.Exec(`
			UPDATE config_versions
			SET config_value=?, changed_by=?, row_version=row_version+1, updated_at=?
			WHERE namespace=? AND config_key=?`,
			value, changedBy, now, namespace, key,
		)
	}
	return err
}

// — scan helpers ——————————————————————————————————————————————————————————————

func scanRow(row *sql.Row) (*domain.ConfigVersion, error) {
	var cv domain.ConfigVersion
	var creAt, updAt string
	err := row.Scan(
		&cv.ID, &cv.Namespace, &cv.ConfigKey, &cv.ConfigValue,
		&cv.Description, &cv.ChangedBy, &cv.RowVersion,
		&creAt, &updAt,
		&cv.ChangedByName,
	)
	if err != nil {
		return nil, err
	}
	cv.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
	cv.UpdatedAt, _ = time.Parse(time.RFC3339, updAt)
	return &cv, nil
}

func scanRows(rows *sql.Rows) ([]domain.ConfigVersion, error) {
	var items []domain.ConfigVersion
	for rows.Next() {
		var cv domain.ConfigVersion
		var creAt, updAt string
		if err := rows.Scan(
			&cv.ID, &cv.Namespace, &cv.ConfigKey, &cv.ConfigValue,
			&cv.Description, &cv.ChangedBy, &cv.RowVersion,
			&creAt, &updAt,
			&cv.ChangedByName,
		); err != nil {
			return nil, err
		}
		cv.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
		cv.UpdatedAt, _ = time.Parse(time.RFC3339, updAt)
		items = append(items, cv)
	}
	return items, rows.Err()
}

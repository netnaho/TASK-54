package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/exams/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// TemplateRepo handles persistence for exam templates.
type TemplateRepo struct {
	db *sql.DB
}

// NewTemplateRepo constructs a TemplateRepo.
func NewTemplateRepo(db *sql.DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

// List returns all active exam templates ordered by title.
func (r *TemplateRepo) List() ([]domain.ExamTemplate, error) {
	rows, err := r.db.Query(`
		SELECT id, title, description, role_scope, passing_score,
		       duration_minutes, window_start, window_end, template_version,
		       is_active, created_by, created_at, updated_at
		FROM competency_exam_templates
		WHERE is_active = 1
		ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("template_repo.List: %w", err)
	}
	defer rows.Close()
	return scanTemplateRows(rows)
}

// ListAll returns all templates regardless of active status.
func (r *TemplateRepo) ListAll() ([]domain.ExamTemplate, error) {
	rows, err := r.db.Query(`
		SELECT id, title, description, role_scope, passing_score,
		       duration_minutes, window_start, window_end, template_version,
		       is_active, created_by, created_at, updated_at
		FROM competency_exam_templates ORDER BY title`)
	if err != nil {
		return nil, fmt.Errorf("template_repo.ListAll: %w", err)
	}
	defer rows.Close()
	return scanTemplateRows(rows)
}

// FindByID returns the template with the given ID or sql.ErrNoRows.
func (r *TemplateRepo) FindByID(id string) (*domain.ExamTemplate, error) {
	row := r.db.QueryRow(`
		SELECT id, title, description, role_scope, passing_score,
		       duration_minutes, window_start, window_end, template_version,
		       is_active, created_by, created_at, updated_at
		FROM competency_exam_templates WHERE id = ?`, id)
	return scanTemplateRow(row)
}

// Create inserts a new exam template. Returns the created template with its ID.
func (r *TemplateRepo) Create(cmd domain.CreateTemplate, createdBy string) (*domain.ExamTemplate, error) {
	id := idgen.New()
	now := timeutil.NowISO()
	_, err := r.db.Exec(`
		INSERT INTO competency_exam_templates
		    (id, title, description, role_scope, passing_score,
		     duration_minutes, window_start, window_end, template_version,
		     is_active, created_by, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,1,1,?,?,?)`,
		id, cmd.Title, cmd.Description, cmd.RoleScope, cmd.PassingScore,
		cmd.DurationMinutes, cmd.WindowStart, cmd.WindowEnd,
		createdBy, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("template_repo.Create: %w", err)
	}
	return r.FindByID(id)
}

// Update applies editable fields and increments template_version.
// currentVersion is checked for optimistic locking; pass 0 to skip the check.
func (r *TemplateRepo) Update(id string, cmd domain.UpdateTemplate, currentVersion int) (*domain.ExamTemplate, error) {
	now := timeutil.NowISO()
	result, err := r.db.Exec(`
		UPDATE competency_exam_templates
		SET title            = ?,
		    description      = ?,
		    duration_minutes = ?,
		    window_start     = ?,
		    window_end       = ?,
		    passing_score    = ?,
		    template_version = template_version + 1,
		    updated_at       = ?
		WHERE id = ? AND (? = 0 OR template_version = ?)`,
		cmd.Title, cmd.Description, cmd.DurationMinutes,
		cmd.WindowStart, cmd.WindowEnd, cmd.PassingScore,
		now, id, currentVersion, currentVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("template_repo.Update: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("template version conflict: template %s was modified by another request", id)
	}
	return r.FindByID(id)
}

// — scan helpers —

func scanTemplateRow(row *sql.Row) (*domain.ExamTemplate, error) {
	var t domain.ExamTemplate
	var isActive int
	var createdAt, updatedAt string
	err := row.Scan(
		&t.ID, &t.Title, &t.Description, &t.RoleScope, &t.PassingScore,
		&t.DurationMinutes, &t.WindowStart, &t.WindowEnd, &t.TemplateVersion,
		&isActive, &t.CreatedBy, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, err
	}
	t.IsActive = isActive == 1
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &t, nil
}

func scanTemplateRows(rows *sql.Rows) ([]domain.ExamTemplate, error) {
	var items []domain.ExamTemplate
	for rows.Next() {
		var t domain.ExamTemplate
		var isActive int
		var createdAt, updatedAt string
		if err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.RoleScope, &t.PassingScore,
			&t.DurationMinutes, &t.WindowStart, &t.WindowEnd, &t.TemplateVersion,
			&isActive, &t.CreatedBy, &createdAt, &updatedAt,
		); err != nil {
			return nil, err
		}
		t.IsActive = isActive == 1
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		items = append(items, t)
	}
	return items, rows.Err()
}

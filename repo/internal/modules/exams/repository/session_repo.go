package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/exams/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// SessionRepo handles persistence for exam sessions and candidates.
type SessionRepo struct {
	db *sql.DB
}

// NewSessionRepo constructs a SessionRepo.
func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// List returns sessions, newest first, up to limit rows (0 = no limit).
func (r *SessionRepo) List(limit int) ([]domain.ExamSession, error) {
	q := `
		SELECT s.id, s.template_id, t.title, s.template_version_snapshot,
		       s.scheduled_by, u.display_name,
		       s.planned_start, s.planned_end,
		       s.proctor_id, COALESCE(p.display_name,'') AS proctor_name,
		       s.room, s.is_draft, s.published_at, s.published_by,
		       s.status, s.row_version, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM competency_exam_candidates c WHERE c.session_id=s.id) AS cand_count,
		       (SELECT COUNT(*) FROM exam_scheduling_conflicts esc
		               WHERE esc.session_id=s.id AND esc.resolved=0 AND esc.is_blocking=1) AS blocking_count
		FROM competency_exam_sessions s
		JOIN competency_exam_templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.scheduled_by
		LEFT JOIN users p ON p.id = s.proctor_id
		ORDER BY s.planned_start DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := r.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("session_repo.List: %w", err)
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// ListForDate returns all non-cancelled sessions whose planned_start falls on the given UTC date.
func (r *SessionRepo) ListForDate(date time.Time) ([]domain.ExamSession, error) {
	dateStr := date.UTC().Format("2006-01-02")
	rows, err := r.db.Query(`
		SELECT s.id, s.template_id, t.title, s.template_version_snapshot,
		       s.scheduled_by, u.display_name,
		       s.planned_start, s.planned_end,
		       s.proctor_id, COALESCE(p.display_name,'') AS proctor_name,
		       s.room, s.is_draft, s.published_at, s.published_by,
		       s.status, s.row_version, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM competency_exam_candidates c WHERE c.session_id=s.id) AS cand_count,
		       (SELECT COUNT(*) FROM exam_scheduling_conflicts esc
		               WHERE esc.session_id=s.id AND esc.resolved=0 AND esc.is_blocking=1) AS blocking_count
		FROM competency_exam_sessions s
		JOIN competency_exam_templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.scheduled_by
		LEFT JOIN users p ON p.id = s.proctor_id
		WHERE s.status != 'cancelled'
		  AND substr(s.planned_start, 1, 10) = ?
		ORDER BY s.planned_start`,
		dateStr,
	)
	if err != nil {
		return nil, fmt.Errorf("session_repo.ListForDate: %w", err)
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// ListAllActive returns all non-cancelled sessions (used for conflict detection).
func (r *SessionRepo) ListAllActive() ([]domain.ExamSession, error) {
	rows, err := r.db.Query(`
		SELECT s.id, s.template_id, t.title, s.template_version_snapshot,
		       s.scheduled_by, u.display_name,
		       s.planned_start, s.planned_end,
		       s.proctor_id, COALESCE(p.display_name,'') AS proctor_name,
		       s.room, s.is_draft, s.published_at, s.published_by,
		       s.status, s.row_version, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM competency_exam_candidates c WHERE c.session_id=s.id) AS cand_count,
		       0 AS blocking_count
		FROM competency_exam_sessions s
		JOIN competency_exam_templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.scheduled_by
		LEFT JOIN users p ON p.id = s.proctor_id
		WHERE s.status != 'cancelled'
		ORDER BY s.planned_start`)
	if err != nil {
		return nil, fmt.Errorf("session_repo.ListAllActive: %w", err)
	}
	defer rows.Close()
	return scanSessionRows(rows)
}

// FindByID returns the session or sql.ErrNoRows.
func (r *SessionRepo) FindByID(id string) (*domain.ExamSession, error) {
	row := r.db.QueryRow(`
		SELECT s.id, s.template_id, t.title, s.template_version_snapshot,
		       s.scheduled_by, u.display_name,
		       s.planned_start, s.planned_end,
		       s.proctor_id, COALESCE(p.display_name,'') AS proctor_name,
		       s.room, s.is_draft, s.published_at, s.published_by,
		       s.status, s.row_version, s.created_at, s.updated_at,
		       (SELECT COUNT(*) FROM competency_exam_candidates c WHERE c.session_id=s.id) AS cand_count,
		       (SELECT COUNT(*) FROM exam_scheduling_conflicts esc
		               WHERE esc.session_id=s.id AND esc.resolved=0 AND esc.is_blocking=1) AS blocking_count
		FROM competency_exam_sessions s
		JOIN competency_exam_templates t ON t.id = s.template_id
		JOIN users u ON u.id = s.scheduled_by
		LEFT JOIN users p ON p.id = s.proctor_id
		WHERE s.id = ?`, id)
	return scanSessionRow(row)
}

// Create inserts a new draft session. Returns the created session.
func (r *SessionRepo) Create(
	templateID, scheduledBy string,
	plannedStart, plannedEnd time.Time,
	room, proctorID string,
	templateVersionSnapshot int,
) (*domain.ExamSession, error) {
	id := idgen.New()
	now := timeutil.NowISO()
	date := plannedStart.UTC().Format("2006-01-02")
	_, err := r.db.Exec(`
		INSERT INTO competency_exam_sessions
		    (id, template_id, scheduled_by, scheduled_date, location,
		     planned_start, planned_end, proctor_id, room,
		     is_draft, template_version_snapshot, status, row_version, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,1,?,?,1,?,?)`,
		id, templateID, scheduledBy, date, room,
		plannedStart.UTC().Format(time.RFC3339),
		plannedEnd.UTC().Format(time.RFC3339),
		nullableString(proctorID),
		room,
		templateVersionSnapshot,
		"scheduled",
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("session_repo.Create: %w", err)
	}
	return r.FindByID(id)
}

// UpdateBlock moves the session's time block. Enforces optimistic locking.
func (r *SessionRepo) UpdateBlock(id string, plannedStart, plannedEnd time.Time, room, proctorID string, rowVersion int) error {
	now := timeutil.NowISO()
	result, err := r.db.Exec(`
		UPDATE competency_exam_sessions
		SET planned_start = ?,
		    planned_end   = ?,
		    room          = ?,
		    location      = ?,
		    proctor_id    = ?,
		    row_version   = row_version + 1,
		    updated_at    = ?
		WHERE id = ? AND row_version = ?`,
		plannedStart.UTC().Format(time.RFC3339),
		plannedEnd.UTC().Format(time.RFC3339),
		room, room,
		nullableString(proctorID),
		now, id, rowVersion,
	)
	if err != nil {
		return fmt.Errorf("session_repo.UpdateBlock: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("row version conflict: session %s was modified by another request (expected version %d)", id, rowVersion)
	}
	return nil
}

// Publish marks a session as published. Enforces optimistic locking.
func (r *SessionRepo) Publish(id, publishedBy string, rowVersion int) error {
	now := timeutil.NowISO()
	result, err := r.db.Exec(`
		UPDATE competency_exam_sessions
		SET is_draft     = 0,
		    published_at = ?,
		    published_by = ?,
		    row_version  = row_version + 1,
		    updated_at   = ?
		WHERE id = ? AND row_version = ? AND is_draft = 1`,
		now, publishedBy, now, id, rowVersion,
	)
	if err != nil {
		return fmt.Errorf("session_repo.Publish: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("publish failed: session %s not found, already published, or version conflict (expected %d)", id, rowVersion)
	}
	return nil
}

// — Candidates —

// ListCandidates returns all candidates for a session.
func (r *SessionRepo) ListCandidates(sessionID string) ([]domain.ExamCandidate, error) {
	rows, err := r.db.Query(`
		SELECT c.id, c.session_id, c.user_id, u.display_name, c.created_at
		FROM competency_exam_candidates c
		JOIN users u ON u.id = c.user_id
		WHERE c.session_id = ?
		ORDER BY u.display_name`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("session_repo.ListCandidates: %w", err)
	}
	defer rows.Close()
	var items []domain.ExamCandidate
	for rows.Next() {
		var c domain.ExamCandidate
		var ca string
		if err := rows.Scan(&c.ID, &c.SessionID, &c.UserID, &c.UserName, &ca); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		items = append(items, c)
	}
	return items, rows.Err()
}

// AllCandidatesBySession returns candidates grouped by session ID (for conflict detection).
func (r *SessionRepo) AllCandidatesBySession() (map[string][]domain.ExamCandidate, error) {
	rows, err := r.db.Query(`
		SELECT c.id, c.session_id, c.user_id, u.display_name, c.created_at
		FROM competency_exam_candidates c
		JOIN users u ON u.id = c.user_id`)
	if err != nil {
		return nil, fmt.Errorf("session_repo.AllCandidatesBySession: %w", err)
	}
	defer rows.Close()

	m := make(map[string][]domain.ExamCandidate)
	for rows.Next() {
		var c domain.ExamCandidate
		var ca string
		if err := rows.Scan(&c.ID, &c.SessionID, &c.UserID, &c.UserName, &ca); err != nil {
			return nil, err
		}
		c.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		m[c.SessionID] = append(m[c.SessionID], c)
	}
	return m, rows.Err()
}

// AddCandidate registers a user as a candidate for a session.
func (r *SessionRepo) AddCandidate(sessionID, userID string) error {
	id := idgen.New()
	now := timeutil.NowISO()
	_, err := r.db.Exec(`
		INSERT OR IGNORE INTO competency_exam_candidates
		    (id, session_id, user_id, score, passed, graded_at, graded_by, notes, created_at)
		VALUES (?,?,?,NULL,NULL,NULL,NULL,'',?)`,
		id, sessionID, userID, now,
	)
	return err
}

// RemoveCandidate removes a candidate from a session.
func (r *SessionRepo) RemoveCandidate(sessionID, userID string) error {
	_, err := r.db.Exec(`
		DELETE FROM competency_exam_candidates WHERE session_id=? AND user_id=?`,
		sessionID, userID)
	return err
}

// — scan helpers —

func scanSessionRow(row *sql.Row) (*domain.ExamSession, error) {
	var s domain.ExamSession
	var isDraft int
	var plannedStart, plannedEnd, createdAt, updatedAt string
	var publishedAt sql.NullString
	var publishedBy sql.NullString
	var proctorID sql.NullString
	err := row.Scan(
		&s.ID, &s.TemplateID, &s.TemplateTitle, &s.TemplateVersionSnapshot,
		&s.ScheduledBy, &s.ScheduledByName,
		&plannedStart, &plannedEnd,
		&proctorID, &s.ProctorName,
		&s.Room, &isDraft, &publishedAt, &publishedBy,
		&s.Status, &s.RowVersion, &createdAt, &updatedAt,
		&s.CandidateCount, &s.BlockingConflictCount,
	)
	if err != nil {
		return nil, err
	}
	s.IsDraft = isDraft == 1
	s.ProctorID = proctorID.String
	s.PublishedBy = publishedBy.String
	s.PlannedStart, _ = time.Parse(time.RFC3339, plannedStart)
	s.PlannedEnd, _ = time.Parse(time.RFC3339, plannedEnd)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	if publishedAt.Valid && publishedAt.String != "" {
		t, err2 := time.Parse(time.RFC3339, publishedAt.String)
		if err2 == nil {
			s.PublishedAt = &t
		}
	}
	return &s, nil
}

func scanSessionRows(rows *sql.Rows) ([]domain.ExamSession, error) {
	var items []domain.ExamSession
	for rows.Next() {
		var s domain.ExamSession
		var isDraft int
		var plannedStart, plannedEnd, createdAt, updatedAt string
		var publishedAt sql.NullString
		var publishedBy sql.NullString
		var proctorID sql.NullString
		if err := rows.Scan(
			&s.ID, &s.TemplateID, &s.TemplateTitle, &s.TemplateVersionSnapshot,
			&s.ScheduledBy, &s.ScheduledByName,
			&plannedStart, &plannedEnd,
			&proctorID, &s.ProctorName,
			&s.Room, &isDraft, &publishedAt, &publishedBy,
			&s.Status, &s.RowVersion, &createdAt, &updatedAt,
			&s.CandidateCount, &s.BlockingConflictCount,
		); err != nil {
			return nil, err
		}
		s.IsDraft = isDraft == 1
		s.ProctorID = proctorID.String
		s.PublishedBy = publishedBy.String
		s.PlannedStart, _ = time.Parse(time.RFC3339, plannedStart)
		s.PlannedEnd, _ = time.Parse(time.RFC3339, plannedEnd)
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		s.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		if publishedAt.Valid && publishedAt.String != "" {
			t, err2 := time.Parse(time.RFC3339, publishedAt.String)
			if err2 == nil {
				s.PublishedAt = &t
			}
		}
		items = append(items, s)
	}
	return items, rows.Err()
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

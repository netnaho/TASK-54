package repository

import (
	"database/sql"
	"fmt"
	"time"

	"careops/clinic/internal/modules/exams/domain"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
)

// ConflictRepo handles persistence for exam_scheduling_conflicts.
type ConflictRepo struct {
	db *sql.DB
}

// NewConflictRepo constructs a ConflictRepo.
func NewConflictRepo(db *sql.DB) *ConflictRepo {
	return &ConflictRepo{db: db}
}

// ListForSession returns all conflicts for a session, newest first.
func (r *ConflictRepo) ListForSession(sessionID string) ([]domain.ExamConflict, error) {
	rows, err := r.db.Query(`
		SELECT esc.id, esc.session_id, esc.conflict_type, esc.is_blocking,
		       esc.entity_id, esc.conflicting_session_id,
		       COALESCE(t.title,'') AS conflicting_title,
		       esc.overlap_start, esc.overlap_end,
		       esc.detail, esc.resolved, esc.resolution_notes, esc.created_at
		FROM exam_scheduling_conflicts esc
		LEFT JOIN competency_exam_sessions cs ON cs.id = esc.conflicting_session_id
		LEFT JOIN competency_exam_templates t  ON t.id  = cs.template_id
		WHERE esc.session_id = ?
		ORDER BY esc.created_at DESC`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("conflict_repo.ListForSession: %w", err)
	}
	defer rows.Close()
	return scanConflictRows(rows)
}

// BlockingUnresolvedCount returns the number of unresolved blocking conflicts for a session.
func (r *ConflictRepo) BlockingUnresolvedCount(sessionID string) (int, error) {
	var n int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM exam_scheduling_conflicts
		WHERE session_id = ? AND resolved = 0 AND is_blocking = 1`, sessionID).Scan(&n)
	return n, err
}

// DeleteForSession removes all conflicts for a session.
// Called before re-running conflict detection after a block update.
func (r *ConflictRepo) DeleteForSession(sessionID string) error {
	_, err := r.db.Exec(`DELETE FROM exam_scheduling_conflicts WHERE session_id = ?`, sessionID)
	return err
}

// BulkInsert inserts multiple conflict records. All must have the same session_id.
func (r *ConflictRepo) BulkInsert(conflicts []domain.ExamConflict) error {
	if len(conflicts) == 0 {
		return nil
	}
	now := timeutil.NowISO()
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("conflict_repo.BulkInsert begin: %w", err)
	}
	stmt, err := tx.Prepare(`
		INSERT INTO exam_scheduling_conflicts
		    (id, session_id, conflict_type, is_blocking, entity_id,
		     conflicting_session_id, overlap_start, overlap_end,
		     detail, resolved, resolution_notes, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,0,'',?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("conflict_repo.BulkInsert prepare: %w", err)
	}
	defer stmt.Close()

	for _, c := range conflicts {
		id := idgen.New()
		var conflictingSessionID interface{}
		if c.ConflictingSessionID != "" {
			conflictingSessionID = c.ConflictingSessionID
		}
		blocking := 0
		if c.IsBlocking {
			blocking = 1
		}
		overlapStart := ""
		overlapEnd := ""
		if !c.OverlapStart.IsZero() {
			overlapStart = c.OverlapStart.UTC().Format(time.RFC3339)
		}
		if !c.OverlapEnd.IsZero() {
			overlapEnd = c.OverlapEnd.UTC().Format(time.RFC3339)
		}
		if _, err := stmt.Exec(
			id, c.SessionID, string(c.ConflictType), blocking,
			c.EntityID, conflictingSessionID,
			overlapStart, overlapEnd,
			c.Detail, now,
		); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("conflict_repo.BulkInsert exec: %w", err)
		}
	}
	return tx.Commit()
}

// MarkResolved marks a conflict as resolved with notes.
func (r *ConflictRepo) MarkResolved(conflictID, notes string) error {
	_, err := r.db.Exec(`
		UPDATE exam_scheduling_conflicts
		SET resolved = 1, resolution_notes = ?
		WHERE id = ?`, notes, conflictID)
	return err
}

// FindByID returns a single conflict.
func (r *ConflictRepo) FindByID(id string) (*domain.ExamConflict, error) {
	rows, err := r.db.Query(`
		SELECT esc.id, esc.session_id, esc.conflict_type, esc.is_blocking,
		       esc.entity_id, esc.conflicting_session_id,
		       COALESCE(t.title,'') AS conflicting_title,
		       esc.overlap_start, esc.overlap_end,
		       esc.detail, esc.resolved, esc.resolution_notes, esc.created_at
		FROM exam_scheduling_conflicts esc
		LEFT JOIN competency_exam_sessions cs ON cs.id = esc.conflicting_session_id
		LEFT JOIN competency_exam_templates t  ON t.id  = cs.template_id
		WHERE esc.id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("conflict_repo.FindByID: %w", err)
	}
	defer rows.Close()
	items, err := scanConflictRows(rows)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	return &items[0], nil
}

// — scan helpers —

func scanConflictRows(rows *sql.Rows) ([]domain.ExamConflict, error) {
	var items []domain.ExamConflict
	for rows.Next() {
		var c domain.ExamConflict
		var isBlocking, resolved int
		var conflictingSessionID sql.NullString
		var overlapStart, overlapEnd, createdAt string
		if err := rows.Scan(
			&c.ID, &c.SessionID, (*string)(&c.ConflictType), &isBlocking,
			&c.EntityID, &conflictingSessionID,
			&c.ConflictingSessionTitle,
			&overlapStart, &overlapEnd,
			&c.Detail, &resolved, &c.ResolutionNotes, &createdAt,
		); err != nil {
			return nil, err
		}
		c.IsBlocking = isBlocking == 1
		c.Resolved = resolved == 1
		c.ConflictingSessionID = conflictingSessionID.String
		c.OverlapStart, _ = time.Parse(time.RFC3339, overlapStart)
		c.OverlapEnd, _ = time.Parse(time.RFC3339, overlapEnd)
		c.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		items = append(items, c)
	}
	return items, rows.Err()
}

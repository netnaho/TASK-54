// Package service implements the exam scheduling workflow.
//
// Time handling contract (explicit, not implicit):
//
//	All time.Time values in this package are UTC.
//	Template window_start / window_end are "HH:MM" strings in facility local time.
//	The scheduler uses tz (*time.Location) to convert window strings to UTC for
//	comparison.  Tests must set tz = time.UTC and use window strings that are
//	valid in UTC (e.g. "08:00"/"17:00") — there is no hidden timezone.
//
// Publish workflow:
//
//  1. Load session; verify is_draft=1 (not already published).
//  2. Check optimistic lock (row_version must match submitted value).
//  3. Check idempotency key; replay cached response if duplicate.
//  4. Count unresolved blocking conflicts; reject with ErrBlockingConflicts if any.
//  5. Persist publish (is_draft=0, published_at, published_by, row_version++).
//  6. Store idempotency record.
//  7. Audit log.
package service

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	auditsvc "careops/clinic/internal/modules/audit/service"
	audithelper "careops/clinic/internal/shared/audit"
	"careops/clinic/internal/modules/exams/domain"
	examrepo "careops/clinic/internal/modules/exams/repository"
	"careops/clinic/internal/platform/idempotency"
)

// ErrBlockingConflicts is returned by Publish when unresolved blocking conflicts exist.
var ErrBlockingConflicts = errors.New("session has unresolved blocking conflicts")

// ErrVersionConflict is returned when an optimistic lock check fails.
var ErrVersionConflict = errors.New("row version conflict")

// ErrAlreadyPublished is returned when trying to publish a non-draft session.
var ErrAlreadyPublished = errors.New("session is already published")

// ExamScheduler orchestrates template management, session generation,
// conflict detection, and publishing.
type ExamScheduler struct {
	templateRepo *examrepo.TemplateRepo
	sessionRepo  *examrepo.SessionRepo
	conflictRepo *examrepo.ConflictRepo
	auditSvc     *auditsvc.AuditService
	idempotency  *idempotency.Service
	tz           *time.Location
	log          *slog.Logger
}

// NewExamScheduler constructs a scheduler.  tz must not be nil; use time.UTC for tests.
func NewExamScheduler(
	templateRepo *examrepo.TemplateRepo,
	sessionRepo *examrepo.SessionRepo,
	conflictRepo *examrepo.ConflictRepo,
	auditSvc *auditsvc.AuditService,
	idp *idempotency.Service,
	tz *time.Location,
	log *slog.Logger,
) *ExamScheduler {
	if tz == nil {
		tz = time.UTC
	}
	return &ExamScheduler{
		templateRepo: templateRepo,
		sessionRepo:  sessionRepo,
		conflictRepo: conflictRepo,
		auditSvc:     auditSvc,
		idempotency:  idp,
		tz:           tz,
		log:          log,
	}
}

// CreateTemplate validates and persists a new exam template.
func (s *ExamScheduler) CreateTemplate(cmd domain.CreateTemplate, createdBy, ipAddress, requestID string) (*domain.ExamTemplate, error) {
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("create_template validation: %w", err)
	}
	tmpl, err := s.templateRepo.Create(cmd, createdBy)
	if err != nil {
		return nil, err
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:        createdBy,
		Action:        "exam_template.create",
		EntityType:    "competency_exam_templates",
		EntityID:      tmpl.ID,
		Details:       fmt.Sprintf("created template %q (version 1)", tmpl.Title),
		AfterSnapshot: audithelper.ToJSON(tmpl),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	return tmpl, nil
}

// UpdateTemplate validates and applies editable fields, bumping template_version.
// currentVersion is the version the caller last read; a mismatch returns ErrVersionConflict.
func (s *ExamScheduler) UpdateTemplate(id string, cmd domain.UpdateTemplate, currentVersion int, updatedBy, ipAddress, requestID string) (*domain.ExamTemplate, error) {
	if err := cmd.Validate(); err != nil {
		return nil, fmt.Errorf("update_template validation: %w", err)
	}
	before, _ := s.templateRepo.FindByID(id)
	tmpl, err := s.templateRepo.Update(id, cmd, currentVersion)
	if err != nil {
		if isVersionConflictMsg(err) {
			return nil, ErrVersionConflict
		}
		return nil, err
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:         updatedBy,
		Action:         "exam_template.update",
		EntityType:     "competency_exam_templates",
		EntityID:       id,
		Details:        fmt.Sprintf("updated template to version %d", tmpl.TemplateVersion),
		BeforeSnapshot: audithelper.ToJSON(before),
		AfterSnapshot:  audithelper.ToJSON(tmpl),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	return tmpl, nil
}

// GenerateSession creates a draft exam session from a template and immediately
// runs conflict detection.  Returns the session and any detected conflicts.
func (s *ExamScheduler) GenerateSession(cmd domain.GenerateSessionCmd, scheduledBy, ipAddress, requestID string) (*domain.ExamSession, []domain.ExamConflict, error) {
	if err := cmd.Validate(); err != nil {
		return nil, nil, fmt.Errorf("generate_session validation: %w", err)
	}

	tmpl, err := s.templateRepo.FindByID(cmd.TemplateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("template %q not found", cmd.TemplateID)
		}
		return nil, nil, err
	}

	plannedEnd := cmd.PlannedStart.Add(time.Duration(tmpl.DurationMinutes) * time.Minute)

	sess, err := s.sessionRepo.Create(
		tmpl.ID, scheduledBy,
		cmd.PlannedStart, plannedEnd,
		cmd.Room, cmd.ProctorID,
		tmpl.TemplateVersion,
	)
	if err != nil {
		return nil, nil, err
	}

	// Register candidates.
	for _, uid := range cmd.CandidateIDs {
		if err := s.sessionRepo.AddCandidate(sess.ID, uid); err != nil {
			s.log.Warn("add candidate failed", "session", sess.ID, "user", uid, "err", err)
		}
	}

	conflicts, err := s.runAndPersistConflicts(sess)
	if err != nil {
		s.log.Warn("conflict detection failed after generate", "session", sess.ID, "err", err)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        scheduledBy,
		Action:        "exam_session.generate",
		EntityType:    "competency_exam_sessions",
		EntityID:      sess.ID,
		Details:       fmt.Sprintf("generated draft session for template %q; %d conflict(s) detected", tmpl.Title, len(conflicts)),
		AfterSnapshot: audithelper.ToJSON(sess),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	return sess, conflicts, nil
}

// UpdateSessionBlock moves a session's time block, re-runs conflict detection,
// and persists the new conflict set.
func (s *ExamScheduler) UpdateSessionBlock(cmd domain.UpdateSessionBlockCmd, updatedBy, ipAddress, requestID string) (*domain.ExamSession, []domain.ExamConflict, error) {
	if err := cmd.Validate(); err != nil {
		return nil, nil, fmt.Errorf("update_block validation: %w", err)
	}

	sess, err := s.sessionRepo.FindByID(cmd.SessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, fmt.Errorf("session %q not found", cmd.SessionID)
		}
		return nil, nil, err
	}
	if !sess.IsDraft {
		return nil, nil, fmt.Errorf("cannot move a published session")
	}

	tmpl, err := s.templateRepo.FindByID(sess.TemplateID)
	if err != nil {
		return nil, nil, err
	}

	plannedEnd := cmd.PlannedStart.Add(time.Duration(tmpl.DurationMinutes) * time.Minute)

	if err := s.sessionRepo.UpdateBlock(
		cmd.SessionID, cmd.PlannedStart, plannedEnd,
		cmd.Room, cmd.ProctorID, cmd.RowVersion,
	); err != nil {
		if isVersionConflictMsg(err) {
			return nil, nil, ErrVersionConflict
		}
		return nil, nil, err
	}

	// Reload with updated values.
	sess, err = s.sessionRepo.FindByID(cmd.SessionID)
	if err != nil {
		return nil, nil, err
	}

	conflicts, err := s.runAndPersistConflicts(sess)
	if err != nil {
		s.log.Warn("conflict detection failed after block update", "session", sess.ID, "err", err)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        updatedBy,
		Action:        "exam_session.update_block",
		EntityType:    "competency_exam_sessions",
		EntityID:      sess.ID,
		Details:       fmt.Sprintf("moved block to %s–%s; %d conflict(s) detected", sess.PlannedStart.Format(time.RFC3339), sess.PlannedEnd.Format(time.RFC3339), len(conflicts)),
		AfterSnapshot: audithelper.ToJSON(sess),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	return sess, conflicts, nil
}

// AddCandidate adds a candidate to a draft session and re-detects conflicts.
func (s *ExamScheduler) AddCandidate(sessionID, userID, actorID, ipAddress, requestID string) error {
	sess, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !sess.IsDraft {
		return fmt.Errorf("cannot add candidates to a published session")
	}
	if err := s.sessionRepo.AddCandidate(sessionID, userID); err != nil {
		return err
	}
	if _, err := s.runAndPersistConflicts(sess); err != nil {
		s.log.Warn("conflict re-detection failed after add candidate", "session", sessionID, "err", err)
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:     actorID,
		Action:     "exam_session.add_candidate",
		EntityType: "competency_exam_sessions",
		EntityID:   sessionID,
		Details:    fmt.Sprintf("added candidate %q to session", userID),
		IPAddress:  ipAddress,
		RequestID:  requestID,
		Outcome:    "success",
	})
	return nil
}

// RemoveCandidate removes a candidate from a draft session and re-detects conflicts.
func (s *ExamScheduler) RemoveCandidate(sessionID, userID, actorID, ipAddress, requestID string) error {
	sess, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return err
	}
	if !sess.IsDraft {
		return fmt.Errorf("cannot remove candidates from a published session")
	}
	if err := s.sessionRepo.RemoveCandidate(sessionID, userID); err != nil {
		return err
	}
	if _, err := s.runAndPersistConflicts(sess); err != nil {
		s.log.Warn("conflict re-detection failed after remove candidate", "session", sessionID, "err", err)
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:     actorID,
		Action:     "exam_session.remove_candidate",
		EntityType: "competency_exam_sessions",
		EntityID:   sessionID,
		Details:    fmt.Sprintf("removed candidate %q from session", userID),
		IPAddress:  ipAddress,
		RequestID:  requestID,
		Outcome:    "success",
	})
	return nil
}

// PublishSession validates that no blocking conflicts remain, then marks the
// session as published.  idempotencyKey may be empty; non-empty keys deduplicate
// duplicate POST requests.
func (s *ExamScheduler) PublishSession(sessionID, publishedBy string, rowVersion int, idempotencyKey, ipAddress, requestID string) error {
	// Idempotency check.
	if idempotencyKey != "" {
		if entry, err := s.idempotency.Check(idempotencyKey); err == nil && entry != nil {
			s.log.Info("publish idempotency replay", "session", sessionID, "key", idempotencyKey)
			return nil
		}
	}

	sess, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session %q not found", sessionID)
		}
		return err
	}
	if !sess.IsDraft {
		return ErrAlreadyPublished
	}
	if sess.RowVersion != rowVersion {
		return ErrVersionConflict
	}

	// Count unresolved blocking conflicts.
	blockingCount, err := s.conflictRepo.BlockingUnresolvedCount(sessionID)
	if err != nil {
		return fmt.Errorf("publish: check conflicts: %w", err)
	}
	if blockingCount > 0 {
		return fmt.Errorf("%w: %d unresolved conflict(s) must be resolved before publishing", ErrBlockingConflicts, blockingCount)
	}

	if err := s.sessionRepo.Publish(sessionID, publishedBy, rowVersion); err != nil {
		if isVersionConflictMsg(err) {
			return ErrVersionConflict
		}
		return err
	}

	// Store idempotency record so duplicate requests replay cleanly.
	if idempotencyKey != "" {
		_ = s.idempotency.Store(idempotencyKey, 303, "/exams/sessions/"+sessionID)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:         publishedBy,
		Action:         "exam_session.publish",
		EntityType:     "competency_exam_sessions",
		EntityID:       sessionID,
		Details:        fmt.Sprintf("published session for template %q", sess.TemplateTitle),
		BeforeSnapshot: audithelper.ToJSON(sess),
		AfterSnapshot:  audithelper.ToJSON(map[string]any{"id": sessionID, "is_draft": false, "published_by": publishedBy}),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	return nil
}

// ResolveConflict marks an individual conflict as resolved.
func (s *ExamScheduler) ResolveConflict(conflictID, notes, resolvedBy, ipAddress, requestID string) error {
	conflict, err := s.conflictRepo.FindByID(conflictID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("conflict %q not found", conflictID)
		}
		return err
	}
	if conflict.Resolved {
		return nil // idempotent
	}
	if err := s.conflictRepo.MarkResolved(conflictID, notes); err != nil {
		return err
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:         resolvedBy,
		Action:         "exam_conflict.resolve",
		EntityType:     "exam_scheduling_conflicts",
		EntityID:       conflictID,
		Details:        fmt.Sprintf("resolved %s conflict; notes: %q", conflict.ConflictType, notes),
		BeforeSnapshot: audithelper.ToJSON(conflict),
		AfterSnapshot:  audithelper.ToJSON(map[string]any{"id": conflictID, "resolved": true, "notes": notes}),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	return nil
}

// RunConflictDetection re-detects and persists conflicts for a session.
// This is idempotent: it clears existing conflict records before writing new ones.
func (s *ExamScheduler) RunConflictDetection(sessionID string) ([]domain.ExamConflict, error) {
	sess, err := s.sessionRepo.FindByID(sessionID)
	if err != nil {
		return nil, err
	}
	return s.runAndPersistConflicts(sess)
}

// — internal helpers —

func (s *ExamScheduler) runAndPersistConflicts(sess *domain.ExamSession) ([]domain.ExamConflict, error) {
	tmpl, err := s.templateRepo.FindByID(sess.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("conflict detection: load template: %w", err)
	}

	allSessions, err := s.sessionRepo.ListAllActive()
	if err != nil {
		return nil, fmt.Errorf("conflict detection: load sessions: %w", err)
	}

	candsBySession, err := s.sessionRepo.AllCandidatesBySession()
	if err != nil {
		return nil, fmt.Errorf("conflict detection: load candidates: %w", err)
	}

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             *sess,
		Template:            *tmpl,
		AllSessions:         allSessions,
		CandidatesBySession: candsBySession,
		TZ:                  s.tz,
	})

	// Tag each conflict with the session ID.
	for i := range conflicts {
		conflicts[i].SessionID = sess.ID
	}

	// Replace conflict records atomically: delete old, insert new.
	if err := s.conflictRepo.DeleteForSession(sess.ID); err != nil {
		return nil, fmt.Errorf("conflict detection: delete old: %w", err)
	}
	if err := s.conflictRepo.BulkInsert(conflicts); err != nil {
		return nil, fmt.Errorf("conflict detection: insert new: %w", err)
	}
	return conflicts, nil
}

// isVersionConflictMsg detects the repo's row-version error message pattern.
func isVersionConflictMsg(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return len(msg) > 15 && (msg[:15] == "row version con" || msg[:12] == "publish fail")
}

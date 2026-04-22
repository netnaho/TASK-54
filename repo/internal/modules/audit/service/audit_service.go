package service

import (
	"log/slog"

	"careops/clinic/internal/modules/audit/domain"
	"careops/clinic/internal/modules/audit/repository"
)

// Entry is the caller-facing write DTO for audit log entries.
// Callers must mask sensitive values before populating Details,
// BeforeSnapshot, or AfterSnapshot (use crypto.MaskString).
type Entry struct {
	UserID         string
	Action         string
	EntityType     string
	EntityID       string
	Details        string
	BeforeSnapshot string
	AfterSnapshot  string
	IPAddress      string
	RequestID      string
	Outcome        string // "success" | "failure" | "error"; defaults to "success"
}

// AuditService wraps the repository and provides fire-and-forget write semantics.
// Audit failures are logged but never propagate to the user-facing request.
type AuditService struct {
	repo *repository.AuditRepo
	log  *slog.Logger
}

func NewAuditService(repo *repository.AuditRepo, log *slog.Logger) *AuditService {
	return &AuditService{repo: repo, log: log}
}

// Record writes an immutable audit entry. Errors are logged but swallowed so that
// an audit write failure does not disrupt the operation being audited.
func (s *AuditService) Record(e Entry) {
	re := repository.RecordEntry{
		UserID:         e.UserID,
		Action:         e.Action,
		EntityType:     e.EntityType,
		EntityID:       e.EntityID,
		Details:        e.Details,
		BeforeSnapshot: e.BeforeSnapshot,
		AfterSnapshot:  e.AfterSnapshot,
		IPAddress:      e.IPAddress,
		RequestID:      e.RequestID,
		Outcome:        e.Outcome,
	}
	if err := s.repo.Record(re); err != nil {
		s.log.Error("audit write failed", "err", err, "action", e.Action)
	}
}

// List returns the most recent audit entries.
func (s *AuditService) List(limit int) ([]domain.AuditLog, error) {
	return s.repo.List(limit)
}

// ListFiltered returns audit entries matching the filter criteria.
func (s *AuditService) ListFiltered(f domain.ListFilter) ([]domain.AuditLog, error) {
	return s.repo.ListFiltered(f)
}

// Count returns the total number of audit log entries.
func (s *AuditService) Count() (int, error) {
	return s.repo.Count()
}

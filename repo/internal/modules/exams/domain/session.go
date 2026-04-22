package domain

import (
	"fmt"
	"strings"
	"time"
)

// ExamSession is a scheduled (or draft) instance of an ExamTemplate.
type ExamSession struct {
	ID                      string
	TemplateID              string
	TemplateTitle           string
	TemplateVersionSnapshot int
	ScheduledBy             string
	ScheduledByName         string
	PlannedStart            time.Time // UTC
	PlannedEnd              time.Time // UTC
	Room                    string
	ProctorID               string
	ProctorName             string
	IsDraft                 bool
	PublishedAt             *time.Time
	PublishedBy             string
	Status                  string // 'scheduled'|'in_progress'|'completed'|'cancelled'
	CandidateCount          int
	BlockingConflictCount   int // unresolved blocking conflicts
	RowVersion              int
	CreatedAt               time.Time
	UpdatedAt               time.Time
	// Populated on detail view only
	Candidates []ExamCandidate
	Conflicts  []ExamConflict
}

// ExamCandidate is a staff member registered for an exam session.
type ExamCandidate struct {
	ID        string
	SessionID string
	UserID    string
	UserName  string
	CreatedAt time.Time
}

// GenerateSessionCmd carries the inputs needed to create a new draft session.
type GenerateSessionCmd struct {
	TemplateID   string
	PlannedStart time.Time // UTC; caller must have converted from facility local
	Room         string
	ProctorID    string
	CandidateIDs []string
}

// UpdateSessionBlockCmd carries the fields needed to move a session's time block.
type UpdateSessionBlockCmd struct {
	SessionID    string
	PlannedStart time.Time // UTC; new start time
	Room         string
	ProctorID    string
	RowVersion   int // optimistic lock; must match current DB value
}

// AddCandidateCmd adds one candidate to a session.
type AddCandidateCmd struct {
	SessionID string
	UserID    string
}

// Validate checks a GenerateSessionCmd for basic correctness.
func (cmd *GenerateSessionCmd) Validate() error {
	if strings.TrimSpace(cmd.TemplateID) == "" {
		return fmt.Errorf("template_id is required")
	}
	if cmd.PlannedStart.IsZero() {
		return fmt.Errorf("planned_start is required")
	}
	if strings.TrimSpace(cmd.Room) == "" {
		return fmt.Errorf("room is required")
	}
	return nil
}

// Validate checks an UpdateSessionBlockCmd.
func (cmd *UpdateSessionBlockCmd) Validate() error {
	if strings.TrimSpace(cmd.SessionID) == "" {
		return fmt.Errorf("session_id is required")
	}
	if cmd.PlannedStart.IsZero() {
		return fmt.Errorf("planned_start is required")
	}
	if strings.TrimSpace(cmd.Room) == "" {
		return fmt.Errorf("room is required")
	}
	return nil
}

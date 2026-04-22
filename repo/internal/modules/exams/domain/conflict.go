package domain

import (
	"fmt"
	"time"
)

// ConflictType classifies what resource is in conflict.
type ConflictType string

const (
	// ConflictRoom: two sessions share the same room with overlapping time windows.
	ConflictRoom ConflictType = "room"
	// ConflictProctor: two sessions share the same proctor with overlapping windows.
	ConflictProctor ConflictType = "proctor"
	// ConflictCandidate: the same user appears as candidate in two overlapping sessions.
	ConflictCandidate ConflictType = "candidate"
	// ConflictWindow: the session's time falls outside the template's allowed window.
	ConflictWindow ConflictType = "window"
)

// ExamConflict records one detected scheduling conflict for a session.
// All conflicts are blocking — a session cannot be published while any
// unresolved conflict exists.
type ExamConflict struct {
	ID                      string
	SessionID               string
	ConflictType            ConflictType
	IsBlocking              bool // always true; field preserved for future severity tiers
	EntityID                string
	EntityLabel             string // human-readable name of the conflicting entity
	ConflictingSessionID    string // empty for ConflictWindow
	ConflictingSessionTitle string // empty for ConflictWindow
	OverlapStart            time.Time
	OverlapEnd              time.Time
	Detail                  string
	Resolved                bool
	ResolutionNotes         string
	CreatedAt               time.Time
}

// IsResourceConflict returns true for room/proctor/candidate (resource-based) conflicts.
func (c *ExamConflict) IsResourceConflict() bool {
	return c.ConflictType == ConflictRoom ||
		c.ConflictType == ConflictProctor ||
		c.ConflictType == ConflictCandidate
}

// ConflictDetectionInput carries all data needed by DetectConflicts.
// Keeping it as a value type makes the function fully testable without a DB.
type ConflictDetectionInput struct {
	Session    ExamSession
	Template   ExamTemplate
	// AllSessions: all non-cancelled sessions in the system except Session itself.
	AllSessions []ExamSession
	// CandidatesBySession: all candidates keyed by session ID.
	CandidatesBySession map[string][]ExamCandidate
	TZ                  *time.Location
}

// DetectConflicts runs the four conflict rules and returns all conflicts found.
// It does NOT persist anything — persistence is the caller's responsibility.
//
// Rules applied (all produce blocking conflicts):
//
//  1. Room:      Session.Room == other.Room  AND  windows overlap  (room must be non-empty)
//  2. Proctor:   Session.ProctorID == other.ProctorID  AND  windows overlap (proctor must be set)
//  3. Candidate: any candidate of Session also appears in other.Candidates  AND  windows overlap
//  4. Window:    Session.PlannedStart < template.WindowStartUTC  OR
//               Session.PlannedEnd   > template.WindowEndUTC
//
// Overlap test: A overlaps B  ⟺  A.Start < B.End  AND  A.End > B.Start
func DetectConflicts(in ConflictDetectionInput) []ExamConflict {
	var conflicts []ExamConflict

	sess := in.Session
	thisCandidates := in.CandidatesBySession[sess.ID]

	for _, other := range in.AllSessions {
		if other.ID == sess.ID {
			continue
		}
		if other.Status == "cancelled" {
			continue
		}
		if !timeWindowsOverlap(sess.PlannedStart, sess.PlannedEnd, other.PlannedStart, other.PlannedEnd) {
			continue
		}

		overlapStart := maxTime(sess.PlannedStart, other.PlannedStart)
		overlapEnd := minTime(sess.PlannedEnd, other.PlannedEnd)

		// Rule 1 — Room conflict
		if sess.Room != "" && other.Room != "" && sess.Room == other.Room {
			conflicts = append(conflicts, ExamConflict{
				SessionID:               sess.ID,
				ConflictType:            ConflictRoom,
				IsBlocking:              true,
				EntityID:                sess.Room,
				EntityLabel:             sess.Room,
				ConflictingSessionID:    other.ID,
				ConflictingSessionTitle: other.TemplateTitle,
				OverlapStart:            overlapStart,
				OverlapEnd:              overlapEnd,
				Detail: fmt.Sprintf(
					"Room %q is also booked for %q (%s) from %s–%s UTC.",
					sess.Room, other.TemplateTitle, other.ID,
					overlapStart.UTC().Format("15:04"), overlapEnd.UTC().Format("15:04"),
				),
			})
		}

		// Rule 2 — Proctor conflict
		if sess.ProctorID != "" && other.ProctorID != "" && sess.ProctorID == other.ProctorID {
			conflicts = append(conflicts, ExamConflict{
				SessionID:               sess.ID,
				ConflictType:            ConflictProctor,
				IsBlocking:              true,
				EntityID:                sess.ProctorID,
				EntityLabel:             sess.ProctorName,
				ConflictingSessionID:    other.ID,
				ConflictingSessionTitle: other.TemplateTitle,
				OverlapStart:            overlapStart,
				OverlapEnd:              overlapEnd,
				Detail: fmt.Sprintf(
					"Proctor is also assigned to %q (%s) from %s–%s UTC.",
					other.TemplateTitle, other.ID,
					overlapStart.UTC().Format("15:04"), overlapEnd.UTC().Format("15:04"),
				),
			})
		}

		// Rule 3 — Candidate conflict
		otherCandidates := in.CandidatesBySession[other.ID]
		for _, tc := range thisCandidates {
			for _, oc := range otherCandidates {
				if tc.UserID == oc.UserID {
					conflicts = append(conflicts, ExamConflict{
						SessionID:               sess.ID,
						ConflictType:            ConflictCandidate,
						IsBlocking:              true,
						EntityID:                tc.UserID,
						EntityLabel:             tc.UserName,
						ConflictingSessionID:    other.ID,
						ConflictingSessionTitle: other.TemplateTitle,
						OverlapStart:            overlapStart,
						OverlapEnd:              overlapEnd,
						Detail: fmt.Sprintf(
							"Candidate %q (%s) is also registered for %q (%s) from %s–%s UTC.",
							tc.UserName, tc.UserID,
							other.TemplateTitle, other.ID,
							overlapStart.UTC().Format("15:04"), overlapEnd.UTC().Format("15:04"),
						),
					})
				}
			}
		}
	}

	// Rule 4 — Window conflict
	if in.TZ != nil && !sess.PlannedStart.IsZero() {
		winStart, errS := in.Template.WindowStartUTC(sess.PlannedStart, in.TZ)
		winEnd, errE := in.Template.WindowEndUTC(sess.PlannedStart, in.TZ)
		if errS == nil && errE == nil {
			if sess.PlannedStart.Before(winStart) || sess.PlannedEnd.After(winEnd) {
				conflicts = append(conflicts, ExamConflict{
					SessionID:    sess.ID,
					ConflictType: ConflictWindow,
					IsBlocking:   true,
					EntityID:     "",
					EntityLabel:  "Scheduling Window",
					OverlapStart: sess.PlannedStart,
					OverlapEnd:   sess.PlannedEnd,
					Detail: fmt.Sprintf(
						"Session time %s–%s falls outside the template's allowed window %s–%s (facility local time).",
						sess.PlannedStart.In(in.TZ).Format("3:04 PM"),
						sess.PlannedEnd.In(in.TZ).Format("3:04 PM"),
						in.Template.WindowStart, in.Template.WindowEnd,
					),
				})
			}
		}
	}

	return conflicts
}

// timeWindowsOverlap returns true when [aStart,aEnd) and [bStart,bEnd) share any time.
// Zero times (unscheduled sessions) do not overlap.
func timeWindowsOverlap(aStart, aEnd, bStart, bEnd time.Time) bool {
	if aStart.IsZero() || aEnd.IsZero() || bStart.IsZero() || bEnd.IsZero() {
		return false
	}
	return aStart.Before(bEnd) && aEnd.After(bStart)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

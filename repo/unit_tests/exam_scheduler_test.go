package unit_tests

import (
	"testing"
	"time"

	"careops/clinic/internal/modules/exams/domain"
)

// — Template validation —

func TestCreateTemplate_Valid(t *testing.T) {
	cmd := domain.CreateTemplate{
		Title:           "Medication Administration",
		RoleScope:       "nurse",
		PassingScore:    80,
		DurationMinutes: 60,
		WindowStart:     "08:00",
		WindowEnd:       "17:00",
	}
	if err := cmd.Validate(); err != nil {
		t.Errorf("expected valid, got: %v", err)
	}
}

func TestCreateTemplate_MissingTitle(t *testing.T) {
	cmd := domain.CreateTemplate{
		RoleScope:       "nurse",
		PassingScore:    80,
		DurationMinutes: 60,
		WindowStart:     "08:00",
		WindowEnd:       "17:00",
	}
	if err := cmd.Validate(); err == nil {
		t.Error("expected validation error for missing title")
	}
}

func TestCreateTemplate_BadWindowFormat(t *testing.T) {
	cmd := domain.CreateTemplate{
		Title:           "T",
		RoleScope:       "nurse",
		PassingScore:    80,
		DurationMinutes: 60,
		WindowStart:     "8:00",   // missing leading zero
		WindowEnd:       "17:00",
	}
	if err := cmd.Validate(); err == nil {
		t.Error("expected validation error for bad window format")
	}
}

func TestCreateTemplate_WindowTooNarrowForDuration(t *testing.T) {
	cmd := domain.CreateTemplate{
		Title:           "T",
		RoleScope:       "nurse",
		PassingScore:    80,
		DurationMinutes: 120,
		WindowStart:     "16:00",
		WindowEnd:       "17:00", // only 60 min window but 120 min exam
	}
	if err := cmd.Validate(); err == nil {
		t.Error("expected validation error for window narrower than duration")
	}
}

func TestCreateTemplate_PassingScoreOutOfRange(t *testing.T) {
	for _, score := range []int{0, 101} {
		cmd := domain.CreateTemplate{
			Title:           "T",
			RoleScope:       "nurse",
			PassingScore:    score,
			DurationMinutes: 60,
			WindowStart:     "08:00",
			WindowEnd:       "17:00",
		}
		if err := cmd.Validate(); err == nil {
			t.Errorf("expected validation error for passing_score=%d", score)
		}
	}
}

// — Conflict detection helpers —

func makeSession(id, room, proctorID string, start, end time.Time) domain.ExamSession {
	return domain.ExamSession{
		ID:        id,
		Room:      room,
		ProctorID: proctorID,
		PlannedStart: start,
		PlannedEnd:   end,
		IsDraft:   true,
		TemplateTitle: "Test Exam",
	}
}

func makeTemplate(windowStart, windowEnd string, durationMinutes int) domain.ExamTemplate {
	return domain.ExamTemplate{
		ID:              "tmpl-x",
		Title:           "Test Exam",
		DurationMinutes: durationMinutes,
		WindowStart:     windowStart,
		WindowEnd:       windowEnd,
	}
}

func t0(h, m int) time.Time {
	return time.Date(2024, 4, 10, h, m, 0, 0, time.UTC)
}

// — Room conflict —

func TestDetectConflicts_RoomConflict(t *testing.T) {
	sess := makeSession("sess-a", "Room B", "", t0(13, 0), t0(14, 0))
	other := makeSession("sess-b", "Room B", "", t0(13, 30), t0(15, 0))
	tmpl := makeTemplate("08:00", "18:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:     sess,
		Template:    tmpl,
		AllSessions: []domain.ExamSession{sess, other},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:          time.UTC,
	})

	found := false
	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictRoom && c.ConflictingSessionID == other.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("expected room conflict with sess-b, got: %+v", conflicts)
	}
}

func TestDetectConflicts_NoRoomConflict_DifferentRooms(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "", t0(13, 0), t0(14, 0))
	other := makeSession("sess-b", "Room B", "", t0(13, 30), t0(15, 0))
	tmpl := makeTemplate("08:00", "18:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess, other},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:                  time.UTC,
	})

	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictRoom {
			t.Errorf("unexpected room conflict: %+v", c)
		}
	}
}

// — Proctor conflict —

func TestDetectConflicts_ProctorConflict(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "proctor-1", t0(13, 0), t0(14, 0))
	other := makeSession("sess-b", "Room B", "proctor-1", t0(13, 30), t0(15, 0))
	tmpl := makeTemplate("08:00", "18:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess, other},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:                  time.UTC,
	})

	found := false
	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictProctor {
			found = true
		}
	}
	if !found {
		t.Errorf("expected proctor conflict, got: %+v", conflicts)
	}
}

// — Candidate conflict —

func TestDetectConflicts_CandidateConflict(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "", t0(13, 0), t0(14, 0))
	other := makeSession("sess-b", "Room B", "", t0(13, 30), t0(15, 0))
	tmpl := makeTemplate("08:00", "18:00", 60)

	cands := map[string][]domain.ExamCandidate{
		"sess-a": {{UserID: "user-1"}},
		"sess-b": {{UserID: "user-1"}},
	}

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess, other},
		CandidatesBySession: cands,
		TZ:                  time.UTC,
	})

	found := false
	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictCandidate && c.EntityID == "user-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected candidate conflict for user-1, got: %+v", conflicts)
	}
}

func TestDetectConflicts_NoCandidateConflict_NoOverlap(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "", t0(9, 0), t0(10, 0))
	other := makeSession("sess-b", "Room B", "", t0(11, 0), t0(12, 0))
	tmpl := makeTemplate("08:00", "18:00", 60)

	cands := map[string][]domain.ExamCandidate{
		"sess-a": {{UserID: "user-1"}},
		"sess-b": {{UserID: "user-1"}},
	}

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess, other},
		CandidatesBySession: cands,
		TZ:                  time.UTC,
	})

	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictCandidate {
			t.Errorf("unexpected candidate conflict: %+v", c)
		}
	}
}

// — Window conflict —

func TestDetectConflicts_WindowConflict_TooEarly(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "", t0(7, 0), t0(8, 0))
	tmpl := makeTemplate("08:00", "17:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:                  time.UTC,
	})

	found := false
	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictWindow {
			found = true
		}
	}
	if !found {
		t.Errorf("expected window conflict for session before allowed window, got: %+v", conflicts)
	}
}

func TestDetectConflicts_NoWindowConflict_ExactlyOnBoundary(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "", t0(8, 0), t0(9, 0))
	tmpl := makeTemplate("08:00", "17:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:                  time.UTC,
	})

	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictWindow {
			t.Errorf("unexpected window conflict at boundary: %+v", c)
		}
	}
}

// — Adjacent sessions do not conflict —

func TestDetectConflicts_NoConflict_AdjacentSessions(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "proctor-1", t0(9, 0), t0(10, 0))
	other := makeSession("sess-b", "Room A", "proctor-1", t0(10, 0), t0(11, 0))
	tmpl := makeTemplate("08:00", "18:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess, other},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:                  time.UTC,
	})

	if len(conflicts) != 0 {
		t.Errorf("adjacent sessions must not conflict; got %d conflict(s): %+v", len(conflicts), conflicts)
	}
}

// — Session with zero times never triggers time-based conflicts —

func TestDetectConflicts_ZeroTimesNoConflict(t *testing.T) {
	sess := makeSession("sess-a", "Room A", "", time.Time{}, time.Time{})
	other := makeSession("sess-b", "Room A", "", time.Time{}, time.Time{})
	tmpl := makeTemplate("08:00", "17:00", 60)

	conflicts := domain.DetectConflicts(domain.ConflictDetectionInput{
		Session:             sess,
		Template:            tmpl,
		AllSessions:         []domain.ExamSession{sess, other},
		CandidatesBySession: map[string][]domain.ExamCandidate{},
		TZ:                  time.UTC,
	})

	for _, c := range conflicts {
		if c.ConflictType == domain.ConflictRoom || c.ConflictType == domain.ConflictProctor || c.ConflictType == domain.ConflictCandidate {
			t.Errorf("zero-time sessions must not trigger overlap conflicts; got: %+v", c)
		}
	}
}

-- Seed 0010: Exam Scheduler Phase 3
-- Enriches existing exam templates with duration/window data.
-- Adds planning fields to existing sessions.
-- Inserts additional sessions that create room, proctor, and candidate conflicts
-- for UI demonstration.  Conflict records are inserted directly here so the
-- scheduler page has data to display without needing to run the service first.
--
-- All planned_start / planned_end values are UTC.
-- FACILITY_TIMEZONE default is "America/New_York" (UTC-5 in winter, UTC-4 in summer).
-- The dates below use April 2024 (EDT = UTC-4), so 09:00 EDT = 13:00 UTC.

-- ── Template enrichment ────────────────────────────────────────────────────

UPDATE competency_exam_templates SET
    duration_minutes = 90,
    window_start     = '08:00',
    window_end       = '17:00',
    template_version = 1
WHERE id = 'tmpl-001';

UPDATE competency_exam_templates SET
    duration_minutes = 60,
    window_start     = '09:00',
    window_end       = '16:00',
    template_version = 1
WHERE id = 'tmpl-002';

UPDATE competency_exam_templates SET
    duration_minutes = 120,
    window_start     = '08:30',
    window_end       = '17:30',
    template_version = 1
WHERE id = 'tmpl-003';

-- ── Enrich existing sessions ───────────────────────────────────────────────
-- sess-001: Infection Control, April 10 09:00–10:30 EDT (13:00–14:30 UTC)
UPDATE competency_exam_sessions SET
    planned_start              = '2024-04-10T13:00:00Z',
    planned_end                = '2024-04-10T14:30:00Z',
    proctor_id                 = 'usr-training',
    room                       = 'Conference Room B',
    is_draft                   = 1,
    template_version_snapshot  = 1
WHERE id = 'sess-001';

-- sess-002: Medication Safety, April 15 13:00–15:00 EDT (17:00–19:00 UTC) — published
UPDATE competency_exam_sessions SET
    planned_start              = '2024-04-15T17:00:00Z',
    planned_end                = '2024-04-15T19:00:00Z',
    proctor_id                 = 'usr-training',
    room                       = 'Nursing Station 1',
    is_draft                   = 0,
    published_at               = '2024-04-01T14:00:00Z',
    published_by               = 'usr-training',
    template_version_snapshot  = 1
WHERE id = 'sess-002';

-- ── Additional sessions (to create conflict scenarios) ─────────────────────

-- sess-003: Resident Rights, April 10 09:30–10:30 EDT (13:30–14:30 UTC)
-- ROOM CONFLICT with sess-001 (same room, overlapping 13:30–14:30)
-- PROCTOR CONFLICT with sess-001 (same proctor, overlapping 13:30–14:30)
INSERT OR IGNORE INTO competency_exam_sessions
    (id, template_id, scheduled_by, scheduled_date, location,
     planned_start, planned_end, proctor_id, room,
     is_draft, template_version_snapshot, status, row_version, created_at, updated_at)
VALUES
    ('sess-003','tmpl-002','usr-training','2024-04-10','Conference Room B',
     '2024-04-10T13:30:00Z','2024-04-10T14:30:00Z','usr-training','Conference Room B',
     1,1,'scheduled',1,'2024-04-01T00:00:00Z','2024-04-01T00:00:00Z');

-- Candidates for sess-001 and sess-003 (overlap on usr-aide → candidate conflict)
INSERT OR IGNORE INTO competency_exam_candidates
    (id, session_id, user_id, score, passed, graded_at, graded_by, notes, created_at)
VALUES
    ('cand-001','sess-001','usr-aide',NULL,NULL,NULL,NULL,'','2024-03-26T00:00:00Z'),
    ('cand-003','sess-003','usr-aide',NULL,NULL,NULL,NULL,'','2024-04-01T00:00:00Z'),
    ('cand-004','sess-003','usr-nurse',NULL,NULL,NULL,NULL,'','2024-04-01T00:00:00Z');

-- sess-004: Medication Safety on April 17 — within window, no conflicts (clean session)
INSERT OR IGNORE INTO competency_exam_sessions
    (id, template_id, scheduled_by, scheduled_date, location,
     planned_start, planned_end, proctor_id, room,
     is_draft, template_version_snapshot, status, row_version, created_at, updated_at)
VALUES
    ('sess-004','tmpl-003','usr-training','2024-04-17','Training Room A',
     '2024-04-17T13:00:00Z','2024-04-17T15:00:00Z','usr-admin','Training Room A',
     1,1,'scheduled',1,'2024-04-02T00:00:00Z','2024-04-02T00:00:00Z');

INSERT OR IGNORE INTO competency_exam_candidates
    (id, session_id, user_id, score, passed, graded_at, graded_by, notes, created_at)
VALUES
    ('cand-002','sess-002','usr-nurse',NULL,NULL,NULL,NULL,'','2024-03-26T00:00:00Z'),
    ('cand-005','sess-004','usr-frontdesk',NULL,NULL,NULL,NULL,'','2024-04-02T00:00:00Z');

-- ── Pre-seeded conflict records for sess-003 ──────────────────────────────
-- These reflect what conflict detection would produce for sess-003.
-- Generated at seed time so the scheduler UI has data to display.

-- Room conflict: sess-003 vs sess-001 (Conference Room B, 13:30–14:30 UTC)
INSERT OR IGNORE INTO exam_scheduling_conflicts
    (id, session_id, conflict_type, is_blocking, entity_id,
     conflicting_session_id, overlap_start, overlap_end,
     detail, resolved, resolution_notes, created_at)
VALUES
    ('esc-001','sess-003','room',1,'Conference Room B',
     'sess-001','2024-04-10T13:30:00Z','2024-04-10T14:30:00Z',
     'Conference Room B is also booked for "Basic Infection Control Competency" (sess-001) from 13:30–14:30 UTC.',
     0,'','2024-04-01T00:00:00Z');

-- Proctor conflict: sess-003 vs sess-001 (usr-training, 13:30–14:30 UTC)
INSERT OR IGNORE INTO exam_scheduling_conflicts
    (id, session_id, conflict_type, is_blocking, entity_id,
     conflicting_session_id, overlap_start, overlap_end,
     detail, resolved, resolution_notes, created_at)
VALUES
    ('esc-002','sess-003','proctor',1,'usr-training',
     'sess-001','2024-04-10T13:30:00Z','2024-04-10T14:30:00Z',
     'Proctor usr-training is also assigned to "Basic Infection Control Competency" (sess-001) from 13:30–14:30 UTC.',
     0,'','2024-04-01T00:00:00Z');

-- Candidate conflict: sess-003 vs sess-001 (usr-aide in both, overlap 13:30–14:30)
INSERT OR IGNORE INTO exam_scheduling_conflicts
    (id, session_id, conflict_type, is_blocking, entity_id,
     conflicting_session_id, overlap_start, overlap_end,
     detail, resolved, resolution_notes, created_at)
VALUES
    ('esc-003','sess-003','candidate',1,'usr-aide',
     'sess-001','2024-04-10T13:30:00Z','2024-04-10T14:30:00Z',
     'Candidate usr-aide is also registered for "Basic Infection Control Competency" (sess-001) from 13:30–14:30 UTC.',
     0,'','2024-04-01T00:00:00Z');

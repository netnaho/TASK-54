-- Migration 0005: Exam Scheduler (Phase 3)
--
-- Time handling contract (document here, enforced in service layer):
--   All datetimes stored as UTC ISO-8601 strings ("2006-01-02T15:04:05Z").
--   Template window_start / window_end are "HH:MM" strings that represent
--   allowed scheduling times in the FACILITY_TIMEZONE (env var; default
--   "America/New_York").  The scheduler service converts these to UTC when
--   validating planned_start / planned_end.
--   All HTML inputs use datetime-local format ("2006-01-02T15:04") presented
--   in facility local time; the handler converts to UTC before persisting.

-- ── Exam template enrichment ────────────────────────────────────────────────

ALTER TABLE competency_exam_templates
    ADD COLUMN duration_minutes INTEGER NOT NULL DEFAULT 60;
-- How long one sitting of this exam takes (minutes).

ALTER TABLE competency_exam_templates
    ADD COLUMN window_start TEXT NOT NULL DEFAULT '08:00';
-- Earliest allowed planned_start for any session using this template
-- (HH:MM in facility local time).

ALTER TABLE competency_exam_templates
    ADD COLUMN window_end TEXT NOT NULL DEFAULT '17:00';
-- Latest allowed planned_end for any session using this template
-- (HH:MM in facility local time).

ALTER TABLE competency_exam_templates
    ADD COLUMN template_version INTEGER NOT NULL DEFAULT 1;
-- Monotonically incremented each time a coordinator edits the template.
-- Sessions record the version at creation time (template_version_snapshot).

-- ── Exam session enrichment ─────────────────────────────────────────────────

ALTER TABLE competency_exam_sessions
    ADD COLUMN planned_start TEXT NOT NULL DEFAULT '';
-- UTC ISO-8601 datetime.  Empty string = not yet scheduled.

ALTER TABLE competency_exam_sessions
    ADD COLUMN planned_end TEXT NOT NULL DEFAULT '';
-- UTC ISO-8601 datetime.  Set to planned_start + duration_minutes.

ALTER TABLE competency_exam_sessions
    ADD COLUMN proctor_id TEXT;
-- user_id of the assigned proctor (NULL = unassigned).

ALTER TABLE competency_exam_sessions
    ADD COLUMN room TEXT NOT NULL DEFAULT '';
-- Free-form room identifier; the same string on two overlapping sessions
-- triggers a 'room' conflict.

ALTER TABLE competency_exam_sessions
    ADD COLUMN is_draft INTEGER NOT NULL DEFAULT 1;
-- 1 = draft (visible to coordinators only).
-- 0 = published (visible to relevant role-scope staff).
-- A session cannot be published while it has unresolved blocking conflicts.

ALTER TABLE competency_exam_sessions
    ADD COLUMN published_at TEXT;
-- UTC ISO-8601; NULL until successfully published.

ALTER TABLE competency_exam_sessions
    ADD COLUMN published_by TEXT;
-- user_id of the coordinator who published; NULL until published.

ALTER TABLE competency_exam_sessions
    ADD COLUMN template_version_snapshot INTEGER NOT NULL DEFAULT 1;
-- The template_version at the moment the session was generated.
-- Used to warn coordinators if the template has since been revised.

-- ── Scheduling conflict table ────────────────────────────────────────────────
--
-- exam_scheduling_conflicts replaces the sparse competency_exam_conflicts table
-- (which remains in schema for backward compatibility but is unused by this module).
--
-- Conflict rules (all are blocking — there is no soft-warning tier):
--
--   'room'      Two non-cancelled sessions share the same non-empty room string
--               and their [planned_start, planned_end) windows overlap.
--
--   'proctor'   Two non-cancelled sessions share the same proctor_id and their
--               time windows overlap.
--
--   'candidate' The same user_id appears as a candidate in two non-cancelled
--               sessions whose time windows overlap.
--
--   'window'    The session's planned_start or planned_end falls outside the
--               template's allowed window on the session's date (after converting
--               window_start / window_end to UTC using FACILITY_TIMEZONE).
--
-- Overlap test: A overlaps B ⟺  A.start < B.end  AND  A.end > A.start

CREATE TABLE IF NOT EXISTS exam_scheduling_conflicts (
    id                     TEXT NOT NULL PRIMARY KEY,
    session_id             TEXT NOT NULL REFERENCES competency_exam_sessions(id),
    conflict_type          TEXT NOT NULL,
    is_blocking            INTEGER NOT NULL DEFAULT 1,
    entity_id              TEXT NOT NULL DEFAULT '',
    conflicting_session_id TEXT REFERENCES competency_exam_sessions(id),
    overlap_start          TEXT NOT NULL DEFAULT '',
    overlap_end            TEXT NOT NULL DEFAULT '',
    detail                 TEXT NOT NULL DEFAULT '',
    resolved               INTEGER NOT NULL DEFAULT 0,
    resolution_notes       TEXT NOT NULL DEFAULT '',
    created_at             TEXT NOT NULL
);

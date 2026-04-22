-- Migration 0007: Reports Phase 2
-- Adds optimistic concurrency control to scheduled_reports so update handlers
-- can detect concurrent edits and return a meaningful conflict response.
ALTER TABLE scheduled_reports ADD COLUMN row_version INTEGER NOT NULL DEFAULT 1;

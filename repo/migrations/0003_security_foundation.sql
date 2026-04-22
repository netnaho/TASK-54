-- Migration 0003: Security foundation
-- Adds session inactivity tracking and richer audit log fields.

-- 15-minute inactivity timeout: updated on every authenticated request.
-- Empty string means "use created_at as baseline" (backward-compat for existing rows).
ALTER TABLE sessions ADD COLUMN last_activity_at TEXT NOT NULL DEFAULT '';

-- Audit log enrichment: before/after snapshots enable change-diff views.
-- Sensitive field values in snapshots MUST be masked before insertion (see crypto.MaskString).
ALTER TABLE audit_logs ADD COLUMN before_snapshot TEXT NOT NULL DEFAULT '';
ALTER TABLE audit_logs ADD COLUMN after_snapshot  TEXT NOT NULL DEFAULT '';
-- outcome: 'success' | 'failure' | 'error'
ALTER TABLE audit_logs ADD COLUMN outcome         TEXT NOT NULL DEFAULT 'success';

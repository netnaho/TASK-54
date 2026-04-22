-- Migration 0001: Initial schema
-- All FK constraints are enforced (PRAGMA foreign_keys = ON is set at connection time).
-- TEXT dates are stored as ISO-8601 UTC: '2024-01-15T08:30:00Z'.
-- row_version is present on entities likely to need optimistic locking in later phases.

-- ============================================================
-- SCHEMA TRACKING
-- ============================================================

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT NOT NULL PRIMARY KEY,
    applied_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS schema_seeds (
    version     TEXT NOT NULL PRIMARY KEY,
    applied_at  TEXT NOT NULL
);

-- ============================================================
-- IDENTITY & ACCESS
-- ============================================================

CREATE TABLE IF NOT EXISTS roles (
    id          TEXT NOT NULL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id              TEXT NOT NULL PRIMARY KEY,
    email           TEXT NOT NULL UNIQUE,
    display_name    TEXT NOT NULL,
    password_hash   TEXT NOT NULL,
    is_active       INTEGER NOT NULL DEFAULT 1,   -- 0 = disabled
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id     TEXT NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    role_id     TEXT NOT NULL REFERENCES roles(id)  ON DELETE CASCADE,
    granted_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT NOT NULL PRIMARY KEY,   -- UUIDv4
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL UNIQUE,        -- SHA-256 hex of the opaque cookie token
    user_agent  TEXT NOT NULL DEFAULT '',
    ip_address  TEXT NOT NULL DEFAULT '',
    expires_at  TEXT NOT NULL,
    created_at  TEXT NOT NULL
);

-- ============================================================
-- FACILITY — BEDS & RESIDENTS
-- ============================================================

CREATE TABLE IF NOT EXISTS beds (
    id              TEXT NOT NULL PRIMARY KEY,
    ward            TEXT NOT NULL,           -- e.g. "East Wing"
    room_number     TEXT NOT NULL,
    bed_label       TEXT NOT NULL,           -- e.g. "A", "B"
    bed_type        TEXT NOT NULL,           -- 'standard' | 'bariatric' | 'icu'
    is_active       INTEGER NOT NULL DEFAULT 1,
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE (ward, room_number, bed_label)
);

CREATE TABLE IF NOT EXISTS residents (
    id              TEXT NOT NULL PRIMARY KEY,
    mrn             TEXT NOT NULL UNIQUE,    -- Medical record number
    first_name      TEXT NOT NULL,
    last_name       TEXT NOT NULL,
    date_of_birth   TEXT NOT NULL,           -- ISO-8601 date YYYY-MM-DD
    gender          TEXT NOT NULL,           -- 'M' | 'F' | 'other'
    primary_diagnosis TEXT NOT NULL DEFAULT '',
    care_level      TEXT NOT NULL DEFAULT 'standard', -- 'standard'|'memory'|'skilled'
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS admissions (
    id              TEXT NOT NULL PRIMARY KEY,
    resident_id     TEXT NOT NULL REFERENCES residents(id),
    bed_id          TEXT NOT NULL REFERENCES beds(id),
    admitted_at     TEXT NOT NULL,
    discharged_at   TEXT,                    -- NULL while still admitted
    discharge_reason TEXT NOT NULL DEFAULT '',
    admitted_by     TEXT NOT NULL REFERENCES users(id),
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS occupancy_snapshots (
    id              TEXT NOT NULL PRIMARY KEY,
    snapshot_date   TEXT NOT NULL,           -- YYYY-MM-DD
    total_beds      INTEGER NOT NULL,
    occupied_beds   INTEGER NOT NULL,
    available_beds  INTEGER NOT NULL,
    occupancy_rate  REAL NOT NULL,           -- 0.0–1.0
    created_at      TEXT NOT NULL
);

-- ============================================================
-- SERVICE DELIVERY
-- ============================================================

CREATE TABLE IF NOT EXISTS service_orders (
    id              TEXT NOT NULL PRIMARY KEY,
    resident_id     TEXT NOT NULL REFERENCES residents(id),
    ordered_by      TEXT NOT NULL REFERENCES users(id),
    service_type    TEXT NOT NULL,           -- 'PT'|'OT'|'ST'|'nursing'|'aide'
    frequency       TEXT NOT NULL,           -- e.g. '5x/week'
    instructions    TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'active', -- 'active'|'completed'|'cancelled'
    start_date      TEXT NOT NULL,
    end_date        TEXT,
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS service_execution_records (
    id              TEXT NOT NULL PRIMARY KEY,
    service_order_id TEXT NOT NULL REFERENCES service_orders(id),
    performed_by    TEXT NOT NULL REFERENCES users(id),
    performed_at    TEXT NOT NULL,
    duration_minutes INTEGER NOT NULL DEFAULT 0,
    notes           TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'completed', -- 'completed'|'missed'|'refused'
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS care_quality_checkpoints (
    id              TEXT NOT NULL PRIMARY KEY,
    resident_id     TEXT NOT NULL REFERENCES residents(id),
    checked_by      TEXT NOT NULL REFERENCES users(id),
    checkpoint_type TEXT NOT NULL,           -- 'comfort'|'safety'|'hygiene'|'nutrition'
    score           INTEGER NOT NULL,        -- 1–5
    notes           TEXT NOT NULL DEFAULT '',
    checked_at      TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS alert_events (
    id              TEXT NOT NULL PRIMARY KEY,
    resident_id     TEXT REFERENCES residents(id),
    alert_type      TEXT NOT NULL,           -- 'fall_risk'|'medication'|'vitals'|'discharge'
    severity        TEXT NOT NULL,           -- 'low'|'medium'|'high'|'critical'
    message         TEXT NOT NULL,
    is_resolved     INTEGER NOT NULL DEFAULT 0,
    resolved_by     TEXT REFERENCES users(id),
    resolved_at     TEXT,
    triggered_at    TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

-- ============================================================
-- EXERCISE LIBRARY
-- ============================================================

CREATE TABLE IF NOT EXISTS exercise_items (
    id              TEXT NOT NULL PRIMARY KEY,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    category        TEXT NOT NULL,           -- 'strength'|'balance'|'flexibility'|'cardio'|'cognitive'
    difficulty      TEXT NOT NULL,           -- 'beginner'|'intermediate'|'advanced'
    duration_minutes INTEGER NOT NULL DEFAULT 0,
    instructions    TEXT NOT NULL DEFAULT '',
    contraindications TEXT NOT NULL DEFAULT '',
    is_active       INTEGER NOT NULL DEFAULT 1,
    created_by      TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS exercise_tags (
    id      TEXT NOT NULL PRIMARY KEY,
    name    TEXT NOT NULL UNIQUE,
    color   TEXT NOT NULL DEFAULT '#6B7280'  -- Tailwind-style hex for UI badge
);

CREATE TABLE IF NOT EXISTS exercise_item_tags (
    exercise_id TEXT NOT NULL REFERENCES exercise_items(id) ON DELETE CASCADE,
    tag_id      TEXT NOT NULL REFERENCES exercise_tags(id)  ON DELETE CASCADE,
    PRIMARY KEY (exercise_id, tag_id)
);

CREATE TABLE IF NOT EXISTS exercise_favorites (
    user_id     TEXT NOT NULL REFERENCES users(id)          ON DELETE CASCADE,
    exercise_id TEXT NOT NULL REFERENCES exercise_items(id) ON DELETE CASCADE,
    created_at  TEXT NOT NULL,
    PRIMARY KEY (user_id, exercise_id)
);

-- ============================================================
-- COMPETENCY EXAMS
-- ============================================================

CREATE TABLE IF NOT EXISTS competency_exam_templates (
    id              TEXT NOT NULL PRIMARY KEY,
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    role_scope      TEXT NOT NULL,           -- which role this exam targets
    passing_score   INTEGER NOT NULL,        -- percentage 0–100
    question_bank   TEXT NOT NULL DEFAULT '[]', -- JSON array of question objects
    is_active       INTEGER NOT NULL DEFAULT 1,
    created_by      TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS competency_exam_sessions (
    id              TEXT NOT NULL PRIMARY KEY,
    template_id     TEXT NOT NULL REFERENCES competency_exam_templates(id),
    scheduled_by    TEXT NOT NULL REFERENCES users(id),
    scheduled_date  TEXT NOT NULL,
    location        TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'scheduled', -- 'scheduled'|'in_progress'|'completed'|'cancelled'
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS competency_exam_candidates (
    id              TEXT NOT NULL PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES competency_exam_sessions(id),
    user_id         TEXT NOT NULL REFERENCES users(id),
    score           INTEGER,                 -- NULL if not yet graded
    passed          INTEGER,                 -- NULL if not yet graded, 0/1 when graded
    graded_at       TEXT,
    graded_by       TEXT REFERENCES users(id),
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS competency_exam_conflicts (
    id              TEXT NOT NULL PRIMARY KEY,
    session_id      TEXT NOT NULL REFERENCES competency_exam_sessions(id),
    user_id         TEXT NOT NULL REFERENCES users(id),
    conflict_reason TEXT NOT NULL,           -- 'scheduling'|'role_mismatch'|'prior_completion'|'other'
    detail          TEXT NOT NULL DEFAULT '',
    reported_at     TEXT NOT NULL,
    resolved        INTEGER NOT NULL DEFAULT 0,
    created_at      TEXT NOT NULL
);

-- ============================================================
-- FINANCE
-- ============================================================

CREATE TABLE IF NOT EXISTS settlement_shifts (
    id              TEXT NOT NULL PRIMARY KEY,
    shift_date      TEXT NOT NULL,
    opened_by       TEXT NOT NULL REFERENCES users(id),
    closed_by       TEXT REFERENCES users(id),
    opened_at       TEXT NOT NULL,
    closed_at       TEXT,
    status          TEXT NOT NULL DEFAULT 'open',  -- 'open'|'closed'|'reconciled'
    notes           TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS payments (
    id              TEXT NOT NULL PRIMARY KEY,
    resident_id     TEXT NOT NULL REFERENCES residents(id),
    shift_id        TEXT REFERENCES settlement_shifts(id),
    payment_method  TEXT NOT NULL,           -- 'cash'|'card'|'insurance'|'bank_transfer'
    amount_cents    INTEGER NOT NULL,        -- stored in cents to avoid floating-point
    currency        TEXT NOT NULL DEFAULT 'USD',
    reference_number TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    recorded_by     TEXT NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL DEFAULT 'completed', -- 'pending'|'completed'|'reversed'
    row_version     INTEGER NOT NULL DEFAULT 1,
    recorded_at     TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS refunds (
    id              TEXT NOT NULL PRIMARY KEY,
    payment_id      TEXT NOT NULL REFERENCES payments(id),
    amount_cents    INTEGER NOT NULL,
    reason          TEXT NOT NULL,
    processed_by    TEXT NOT NULL REFERENCES users(id),
    processed_at    TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS imported_card_batches (
    id              TEXT NOT NULL PRIMARY KEY,
    file_name       TEXT NOT NULL,
    imported_by     TEXT NOT NULL REFERENCES users(id),
    record_count    INTEGER NOT NULL DEFAULT 0,
    success_count   INTEGER NOT NULL DEFAULT 0,
    error_count     INTEGER NOT NULL DEFAULT 0,
    status          TEXT NOT NULL DEFAULT 'pending', -- 'pending'|'processing'|'complete'|'failed'
    error_detail    TEXT NOT NULL DEFAULT '',
    imported_at     TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

-- ============================================================
-- AUDIT & COMPLIANCE
-- ============================================================

CREATE TABLE IF NOT EXISTS audit_logs (
    id              TEXT NOT NULL PRIMARY KEY,
    user_id         TEXT REFERENCES users(id), -- NULL for system actions
    action          TEXT NOT NULL,             -- 'login'|'logout'|'create'|'update'|'delete'|'view'
    entity_type     TEXT NOT NULL DEFAULT '',
    entity_id       TEXT NOT NULL DEFAULT '',
    details         TEXT NOT NULL DEFAULT '{}', -- JSON object
    ip_address      TEXT NOT NULL DEFAULT '',
    request_id      TEXT NOT NULL DEFAULT '',
    occurred_at     TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key             TEXT NOT NULL PRIMARY KEY,
    response_status INTEGER NOT NULL,
    response_body   TEXT NOT NULL DEFAULT '',
    created_at      TEXT NOT NULL,
    expires_at      TEXT NOT NULL
);

-- ============================================================
-- REPORTING
-- ============================================================

CREATE TABLE IF NOT EXISTS scheduled_reports (
    id              TEXT NOT NULL PRIMARY KEY,
    name            TEXT NOT NULL,
    report_type     TEXT NOT NULL,           -- 'occupancy'|'service_delivery'|'finance'|'compliance'
    schedule_cron   TEXT NOT NULL,           -- cron expression, not yet processed
    parameters      TEXT NOT NULL DEFAULT '{}', -- JSON
    output_format   TEXT NOT NULL DEFAULT 'pdf', -- 'pdf'|'csv'|'json'
    recipients      TEXT NOT NULL DEFAULT '[]',  -- JSON array of emails (future use)
    is_active       INTEGER NOT NULL DEFAULT 1,
    created_by      TEXT NOT NULL REFERENCES users(id),
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS report_runs (
    id              TEXT NOT NULL PRIMARY KEY,
    scheduled_report_id TEXT REFERENCES scheduled_reports(id),
    report_type     TEXT NOT NULL,
    triggered_by    TEXT REFERENCES users(id),
    parameters      TEXT NOT NULL DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending', -- 'pending'|'running'|'complete'|'failed'
    output_path     TEXT NOT NULL DEFAULT '',
    error_message   TEXT NOT NULL DEFAULT '',
    started_at      TEXT,
    completed_at    TEXT,
    created_at      TEXT NOT NULL
);

-- ============================================================
-- SYSTEM OPERATIONS
-- ============================================================

CREATE TABLE IF NOT EXISTS config_versions (
    id              TEXT NOT NULL PRIMARY KEY,
    namespace       TEXT NOT NULL,           -- e.g. 'facility'|'auth'|'finance'
    config_key      TEXT NOT NULL,
    config_value    TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    changed_by      TEXT NOT NULL REFERENCES users(id),
    row_version     INTEGER NOT NULL DEFAULT 1,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE (namespace, config_key)
);

CREATE TABLE IF NOT EXISTS diagnostic_exports (
    id              TEXT NOT NULL PRIMARY KEY,
    export_type     TEXT NOT NULL,           -- 'full'|'db_stats'|'logs'|'config'
    triggered_by    TEXT NOT NULL REFERENCES users(id),
    file_path       TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending',
    error_message   TEXT NOT NULL DEFAULT '',
    triggered_at    TEXT NOT NULL,
    completed_at    TEXT,
    created_at      TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS job_history (
    id              TEXT NOT NULL PRIMARY KEY,
    job_name        TEXT NOT NULL,           -- e.g. 'occupancy_snapshot'|'report_run'
    job_type        TEXT NOT NULL,           -- 'scheduled'|'on_demand'
    status          TEXT NOT NULL,           -- 'success'|'failed'|'skipped'
    duration_ms     INTEGER NOT NULL DEFAULT 0,
    detail          TEXT NOT NULL DEFAULT '',
    ran_at          TEXT NOT NULL,
    created_at      TEXT NOT NULL
);

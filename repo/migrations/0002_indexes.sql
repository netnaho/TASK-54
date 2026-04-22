-- Migration 0002: Non-primary-key indexes for query performance

-- Users
CREATE INDEX IF NOT EXISTS idx_users_email           ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_is_active        ON users(is_active);

-- Sessions
CREATE INDEX IF NOT EXISTS idx_sessions_user_id      ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash   ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at   ON sessions(expires_at);

-- User roles
CREATE INDEX IF NOT EXISTS idx_user_roles_user_id    ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id    ON user_roles(role_id);

-- Beds
CREATE INDEX IF NOT EXISTS idx_beds_ward             ON beds(ward);
CREATE INDEX IF NOT EXISTS idx_beds_is_active        ON beds(is_active);

-- Residents
CREATE INDEX IF NOT EXISTS idx_residents_mrn         ON residents(mrn);
CREATE INDEX IF NOT EXISTS idx_residents_last_name   ON residents(last_name);
CREATE INDEX IF NOT EXISTS idx_residents_care_level  ON residents(care_level);

-- Admissions
CREATE INDEX IF NOT EXISTS idx_admissions_resident   ON admissions(resident_id);
CREATE INDEX IF NOT EXISTS idx_admissions_bed        ON admissions(bed_id);
CREATE INDEX IF NOT EXISTS idx_admissions_admitted   ON admissions(admitted_at);
CREATE INDEX IF NOT EXISTS idx_admissions_discharged ON admissions(discharged_at);

-- Occupancy snapshots
CREATE INDEX IF NOT EXISTS idx_occ_snap_date         ON occupancy_snapshots(snapshot_date);

-- Service orders
CREATE INDEX IF NOT EXISTS idx_so_resident           ON service_orders(resident_id);
CREATE INDEX IF NOT EXISTS idx_so_status             ON service_orders(status);
CREATE INDEX IF NOT EXISTS idx_so_ordered_by         ON service_orders(ordered_by);

-- Execution records
CREATE INDEX IF NOT EXISTS idx_ser_order_id          ON service_execution_records(service_order_id);
CREATE INDEX IF NOT EXISTS idx_ser_performed_by      ON service_execution_records(performed_by);
CREATE INDEX IF NOT EXISTS idx_ser_performed_at      ON service_execution_records(performed_at);

-- Quality checkpoints
CREATE INDEX IF NOT EXISTS idx_cqc_resident          ON care_quality_checkpoints(resident_id);
CREATE INDEX IF NOT EXISTS idx_cqc_checked_at        ON care_quality_checkpoints(checked_at);

-- Alert events
CREATE INDEX IF NOT EXISTS idx_alert_resident        ON alert_events(resident_id);
CREATE INDEX IF NOT EXISTS idx_alert_is_resolved     ON alert_events(is_resolved);
CREATE INDEX IF NOT EXISTS idx_alert_severity        ON alert_events(severity);

-- Exercise items
CREATE INDEX IF NOT EXISTS idx_exercise_category     ON exercise_items(category);
CREATE INDEX IF NOT EXISTS idx_exercise_difficulty   ON exercise_items(difficulty);
CREATE INDEX IF NOT EXISTS idx_exercise_is_active    ON exercise_items(is_active);

-- Exercise relations
CREATE INDEX IF NOT EXISTS idx_eit_exercise_id       ON exercise_item_tags(exercise_id);
CREATE INDEX IF NOT EXISTS idx_ef_user_id            ON exercise_favorites(user_id);

-- Exam sessions
CREATE INDEX IF NOT EXISTS idx_exam_sess_template    ON competency_exam_sessions(template_id);
CREATE INDEX IF NOT EXISTS idx_exam_sess_status      ON competency_exam_sessions(status);

-- Exam candidates
CREATE INDEX IF NOT EXISTS idx_exam_cand_session     ON competency_exam_candidates(session_id);
CREATE INDEX IF NOT EXISTS idx_exam_cand_user        ON competency_exam_candidates(user_id);

-- Payments
CREATE INDEX IF NOT EXISTS idx_payment_resident      ON payments(resident_id);
CREATE INDEX IF NOT EXISTS idx_payment_shift         ON payments(shift_id);
CREATE INDEX IF NOT EXISTS idx_payment_status        ON payments(status);
CREATE INDEX IF NOT EXISTS idx_payment_recorded_at   ON payments(recorded_at);

-- Audit logs
CREATE INDEX IF NOT EXISTS idx_audit_user_id         ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_entity          ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_occurred_at     ON audit_logs(occurred_at);
CREATE INDEX IF NOT EXISTS idx_audit_action          ON audit_logs(action);

-- Idempotency keys
CREATE INDEX IF NOT EXISTS idx_idem_expires_at       ON idempotency_keys(expires_at);

-- Scheduled reports
CREATE INDEX IF NOT EXISTS idx_sched_report_type     ON scheduled_reports(report_type);
CREATE INDEX IF NOT EXISTS idx_sched_report_active   ON scheduled_reports(is_active);

-- Report runs
CREATE INDEX IF NOT EXISTS idx_report_run_type       ON report_runs(report_type);
CREATE INDEX IF NOT EXISTS idx_report_run_status     ON report_runs(status);

-- Config versions
CREATE INDEX IF NOT EXISTS idx_config_namespace      ON config_versions(namespace);

-- Job history
CREATE INDEX IF NOT EXISTS idx_job_name              ON job_history(job_name);
CREATE INDEX IF NOT EXISTS idx_job_ran_at            ON job_history(ran_at);
CREATE INDEX IF NOT EXISTS idx_job_status            ON job_history(status);

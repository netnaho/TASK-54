-- Migration 0006: Finance Phase 4
-- Adds columns required by offline-payment workflows, card-batch imports,
-- shift-close settlement, and observable job execution history.

-- payments: store AES-encrypted reference numbers alongside the masked display
-- value, and link card-terminal payments back to their import batch.
ALTER TABLE payments ADD COLUMN encrypted_reference TEXT NOT NULL DEFAULT '';
ALTER TABLE payments ADD COLUMN batch_id            TEXT REFERENCES imported_card_batches(id);

-- settlement_shifts: track which named shift (morning/afternoon/night) was
-- closed and cache the reconciled totals so exports do not need to re-aggregate.
ALTER TABLE settlement_shifts ADD COLUMN shift_name          TEXT NOT NULL DEFAULT 'morning';
ALTER TABLE settlement_shifts ADD COLUMN total_amount_cents  INTEGER NOT NULL DEFAULT 0;
ALTER TABLE settlement_shifts ADD COLUMN reconciliation_notes TEXT NOT NULL DEFAULT '';

-- imported_card_batches: persist the path where the uploaded file was stored
-- so the file can be re-inspected without re-uploading.
ALTER TABLE imported_card_batches ADD COLUMN file_path TEXT NOT NULL DEFAULT '';
ALTER TABLE imported_card_batches ADD COLUMN shift_id  TEXT REFERENCES settlement_shifts(id);

-- job_history: distinguish user-triggered vs system-triggered jobs and allow
-- correlation with request IDs from HTMX flows.
ALTER TABLE job_history ADD COLUMN actor          TEXT NOT NULL DEFAULT 'system';
ALTER TABLE job_history ADD COLUMN correlation_id TEXT NOT NULL DEFAULT '';

-- report_runs: track the MIME type alongside the output path so download
-- handlers can set the correct Content-Type without guessing from extension.
ALTER TABLE report_runs ADD COLUMN output_format TEXT NOT NULL DEFAULT 'csv';

-- Indexes for the new foreign keys.
CREATE INDEX IF NOT EXISTS idx_payments_batch_id   ON payments(batch_id);
CREATE INDEX IF NOT EXISTS idx_shifts_shift_name   ON settlement_shifts(shift_name, shift_date);
CREATE INDEX IF NOT EXISTS idx_batches_shift_id    ON imported_card_batches(shift_id);
CREATE INDEX IF NOT EXISTS idx_job_history_actor   ON job_history(actor, ran_at);

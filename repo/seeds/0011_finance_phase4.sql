-- Seed 0011: Finance Phase 4 — additional sample data for new workflows

-- A closed shift to demonstrate settlement history
INSERT OR IGNORE INTO settlement_shifts
    (id, shift_date, shift_name, opened_by, closed_by, opened_at, closed_at,
     status, notes, total_amount_cents, reconciliation_notes, created_at, updated_at)
VALUES
  ('shift-002','2024-04-01','afternoon',
   'usr-finance','usr-finance',
   '2024-04-01T15:00:00Z','2024-04-01T23:00:00Z',
   'closed',
   'Afternoon shift — closed without issues',
   37500,
   'Cash: $125.00 (1 tx); Card batch TXN-B-0042 processed.',
   '2024-04-01T15:00:00Z','2024-04-01T23:00:00Z');

-- Payments linked to the closed shift
INSERT OR IGNORE INTO payments
    (id, resident_id, shift_id, payment_method, amount_cents, currency,
     reference_number, encrypted_reference, description, recorded_by,
     status, row_version, recorded_at, created_at, updated_at)
VALUES
  ('pay-004','res-003','shift-002','cash',12500,'USD','RCPT-20240401-4','',
   'Afternoon cash co-pay','usr-finance','completed',1,
   '2024-04-01T16:00:00Z','2024-04-01T16:00:00Z','2024-04-01T16:00:00Z'),
  ('pay-005','res-009','shift-002','check',25000,'USD','CHK-4821','',
   'April private pay — check 4821','usr-finance','completed',1,
   '2024-04-01T17:30:00Z','2024-04-01T17:30:00Z','2024-04-01T17:30:00Z');

-- A sample refund
INSERT OR IGNORE INTO refunds
    (id, payment_id, amount_cents, reason, processed_by, processed_at, created_at)
VALUES
  ('ref-001','pay-001',5000,
   'Overpayment — insurance adjustment reversed partial amount',
   'usr-finance','2024-04-02T10:00:00Z','2024-04-02T10:00:00Z');

-- A sample completed card-batch import
INSERT OR IGNORE INTO imported_card_batches
    (id, file_name, file_path, imported_by, shift_id,
     record_count, success_count, error_count,
     status, error_detail, imported_at, created_at)
VALUES
  ('batch-001','terminal_20240402_am.csv','',
   'usr-finance','shift-001',
   5,5,0,
   'complete','',
   '2024-04-02T08:45:00Z','2024-04-02T08:45:00Z');

-- A sample scheduled report definition
INSERT OR IGNORE INTO scheduled_reports
    (id, name, report_type, schedule_cron, parameters, output_format,
     recipients, is_active, created_by, created_at, updated_at)
VALUES
  ('rpt-def-001',
   'Daily Finance Summary',
   'finance',
   '0 23 * * *',
   '{"date_range":"today"}',
   'csv',
   '[]',
   1,
   'usr-admin',
   '2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('rpt-def-002',
   'Weekly Occupancy Report',
   'occupancy',
   '0 6 * * 1',
   '{"weeks":1}',
   'csv',
   '[]',
   1,
   'usr-admin',
   '2024-01-01T00:00:00Z','2024-01-01T00:00:00Z');

-- Sample report runs (historical, both complete)
INSERT OR IGNORE INTO report_runs
    (id, scheduled_report_id, report_type, triggered_by, parameters,
     status, output_path, output_format, error_message,
     started_at, completed_at, created_at)
VALUES
  ('rrun-001','rpt-def-001','finance','usr-admin',
   '{"date_range":"2024-04-01"}',
   'complete',
   'exports/finance_summary_2024-04-01.csv',
   'csv','',
   '2024-04-01T23:00:05Z','2024-04-01T23:00:08Z','2024-04-01T23:00:05Z'),
  ('rrun-002','rpt-def-002','occupancy','usr-admin',
   '{"date_from":"2024-03-25","date_to":"2024-04-01"}',
   'complete',
   'exports/occupancy_2024-03-25_2024-04-01.csv',
   'csv','',
   '2024-04-01T06:00:05Z','2024-04-01T06:00:07Z','2024-04-01T06:00:05Z');

-- Sample diagnostic exports
INSERT OR IGNORE INTO diagnostic_exports
    (id, export_type, triggered_by, file_path, status, error_message,
     triggered_at, completed_at, created_at)
VALUES
  ('diag-001','full','usr-admin',
   'diagnostics/careops_diag_20240401_070100.zip',
   'complete','',
   '2024-04-01T07:01:00Z','2024-04-01T07:01:12Z','2024-04-01T07:01:00Z');

-- Updated job history entries for phase 4 operations (with actor column)
INSERT OR IGNORE INTO job_history
    (id, job_name, job_type, status, duration_ms, detail,
     actor, correlation_id, ran_at, created_at)
VALUES
  ('job-004','finance_export','on_demand','success',340,
   'Finance summary CSV generated: exports/finance_summary_2024-04-01.csv',
   'usr-admin','','2024-04-01T23:00:08Z','2024-04-01T23:00:08Z'),
  ('job-005','card_batch_import','on_demand','success',210,
   'Batch terminal_20240402_am.csv: 5 records, 5 ok, 0 errors',
   'usr-finance','','2024-04-02T08:45:10Z','2024-04-02T08:45:10Z'),
  ('job-006','diagnostic_export','on_demand','success',1240,
   'Diagnostic package: diagnostics/careops_diag_20240401_070100.zip (42 KB)',
   'usr-admin','','2024-04-01T07:01:12Z','2024-04-01T07:01:12Z'),
  ('job-007','shift_close','on_demand','success',55,
   'Shift shift-002 (afternoon 2024-04-01) closed. Total: $375.00 across 2 payments.',
   'usr-finance','','2024-04-01T23:00:00Z','2024-04-01T23:00:00Z');

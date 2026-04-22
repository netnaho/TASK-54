-- Seed 0008: System configuration and sample finance records

-- Config versions — facility and system settings
INSERT OR IGNORE INTO config_versions (id, namespace, config_key, config_value, description, changed_by, row_version, created_at, updated_at) VALUES
  ('cfg-001','facility','name',         'Sunridge Rehab & Long-Term Care', 'Facility legal name',             'usr-admin',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('cfg-002','facility','address',      '400 Sunridge Drive, Maplewood, OR 97001', 'Facility mailing address','usr-admin',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('cfg-003','facility','phone',        '(503) 555-0100',                  'Main facility phone number',      'usr-admin',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('cfg-004','facility','license_number','OR-LTC-2019-0042',              'State operating license number',  'usr-admin',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('cfg-005','auth',    'session_ttl_hours','24',                          'Session cookie lifetime in hours','usr-admin',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('cfg-006','finance', 'currency',     'USD',                             'Default currency for payments',   'usr-admin',1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z');

-- Sample open settlement shift
INSERT OR IGNORE INTO settlement_shifts (id, shift_date, opened_by, closed_by, opened_at, closed_at, status, notes, created_at, updated_at) VALUES
  ('shift-001','2024-04-01','usr-finance', NULL,'2024-04-01T07:00:00Z', NULL,'open','Day shift — morning open','2024-04-01T07:00:00Z','2024-04-01T07:00:00Z');

-- Sample payments
INSERT OR IGNORE INTO payments (id, resident_id, shift_id, payment_method, amount_cents, currency, reference_number, description, recorded_by, status, row_version, recorded_at, created_at, updated_at) VALUES
  ('pay-001','res-001','shift-001','insurance', 45000,'USD','INS-2024-03-001','March co-pay — Medicare Part A','usr-finance',   'completed',1,'2024-04-01T08:15:00Z','2024-04-01T08:15:00Z','2024-04-01T08:15:00Z'),
  ('pay-002','res-005','shift-001','card',       12500,'USD','CARD-TXN-98312', 'April room fee partial payment','usr-finance',  'completed',1,'2024-04-01T09:30:00Z','2024-04-01T09:30:00Z','2024-04-01T09:30:00Z'),
  ('pay-003','res-007','shift-001','cash',        5000,'USD','RCPT-20240401-3','Private pay co-pay April',      'usr-finance',  'completed',1,'2024-04-01T10:00:00Z','2024-04-01T10:00:00Z','2024-04-01T10:00:00Z');

-- Sample service orders
INSERT OR IGNORE INTO service_orders (id, resident_id, ordered_by, service_type, frequency, instructions, status, start_date, end_date, row_version, created_at, updated_at) VALUES
  ('so-001','res-001','usr-nurse','PT','5x/week','Focus on weight bearing and gait training post right hip ORIF.','active','2024-01-06',NULL,1,'2024-01-06T00:00:00Z','2024-01-06T00:00:00Z'),
  ('so-002','res-004','usr-nurse','OT','3x/week','ADL retraining following left CVA with right hemiplegia.','active','2024-01-13',NULL,1,'2024-01-13T00:00:00Z','2024-01-13T00:00:00Z'),
  ('so-003','res-009','usr-nurse','PT','3x/week','Lumbar stabilization and pain management for spinal stenosis.','active','2024-03-11',NULL,1,'2024-03-11T00:00:00Z','2024-03-11T00:00:00Z'),
  ('so-004','res-005','usr-nurse','PT','5x/week','Knee ROM and strengthening following total knee arthroplasty.','active','2024-02-02',NULL,1,'2024-02-02T00:00:00Z','2024-02-02T00:00:00Z');

-- Sample execution records
INSERT OR IGNORE INTO service_execution_records (id, service_order_id, performed_by, performed_at, duration_minutes, notes, status, created_at) VALUES
  ('ser-001','so-001','usr-therapist','2024-04-01T09:00:00Z',45,'Resident ambulated 50 ft with rolling walker. Tolerates activity well. No c/o pain.','completed','2024-04-01T09:50:00Z'),
  ('ser-002','so-002','usr-therapist','2024-04-01T10:00:00Z',40,'Practiced shirt donning/doffing. Used button hook. 60% independence achieved.','completed','2024-04-01T10:45:00Z'),
  ('ser-003','so-001','usr-therapist','2024-04-02T09:15:00Z',45,'Resident refused session, reported fatigue. Nursing notified.','refused','2024-04-02T09:20:00Z');

-- Sample audit log entries
INSERT OR IGNORE INTO audit_logs (id, user_id, action, entity_type, entity_id, details, ip_address, request_id, occurred_at, created_at) VALUES
  ('alog-001','usr-admin',  'login', '',        '',        '{"method":"password"}', '192.168.1.10','req-seed-001','2024-04-01T07:01:00Z','2024-04-01T07:01:00Z'),
  ('alog-002','usr-nurse',  'login', '',        '',        '{"method":"password"}', '192.168.1.15','req-seed-002','2024-04-01T07:05:00Z','2024-04-01T07:05:00Z'),
  ('alog-003','usr-nurse',  'view',  'resident','res-001', '{}',                    '192.168.1.15','req-seed-003','2024-04-01T07:10:00Z','2024-04-01T07:10:00Z'),
  ('alog-004','usr-finance','login', '',        '',        '{"method":"password"}', '192.168.1.20','req-seed-004','2024-04-01T07:30:00Z','2024-04-01T07:30:00Z'),
  ('alog-005','usr-finance','create','payment', 'pay-001', '{"amount_cents":45000}','192.168.1.20','req-seed-005','2024-04-01T08:15:00Z','2024-04-01T08:15:00Z');

-- Sample job history
INSERT OR IGNORE INTO job_history (id, job_name, job_type, status, duration_ms, detail, ran_at, created_at) VALUES
  ('job-001','occupancy_snapshot','scheduled','success',120,'Snapshot recorded: 10/18 occupied','2024-04-01T00:05:00Z','2024-04-01T00:05:00Z'),
  ('job-002','occupancy_snapshot','scheduled','success', 95,'Snapshot recorded: 10/18 occupied','2024-03-31T00:05:00Z','2024-03-31T00:05:00Z'),
  ('job-003','occupancy_snapshot','scheduled','success',110,'Snapshot recorded: 9/18 occupied', '2024-03-30T00:05:00Z','2024-03-30T00:05:00Z');

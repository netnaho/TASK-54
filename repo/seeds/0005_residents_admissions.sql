-- Seed 0005: Residents and admissions

INSERT OR IGNORE INTO residents (id, mrn, first_name, last_name, date_of_birth, gender, primary_diagnosis, care_level, row_version, created_at, updated_at) VALUES
  ('res-001','MRN-00001','Margaret','Holloway','1938-06-12','F','Hip fracture post-repair','skilled',    1,'2024-01-05T09:00:00Z','2024-01-05T09:00:00Z'),
  ('res-002','MRN-00002','Robert',  'Chen',     '1942-11-28','M','COPD exacerbation',       'standard',  1,'2024-01-07T10:30:00Z','2024-01-07T10:30:00Z'),
  ('res-003','MRN-00003','Eleanor', 'Vasquez',  '1935-03-04','F','Alzheimers stage 3',      'memory',    1,'2024-01-10T08:15:00Z','2024-01-10T08:15:00Z'),
  ('res-004','MRN-00004','Harold',  'Patel',    '1940-09-19','M','Stroke recovery',         'skilled',   1,'2024-01-12T11:00:00Z','2024-01-12T11:00:00Z'),
  ('res-005','MRN-00005','Dorothy', 'Williams', '1948-07-23','F','Knee replacement rehab',  'standard',  1,'2024-02-01T14:00:00Z','2024-02-01T14:00:00Z'),
  ('res-006','MRN-00006','Walter',  'Johnson',  '1933-02-14','M','Parkinson''s disease',    'memory',    1,'2024-02-05T09:45:00Z','2024-02-05T09:45:00Z'),
  ('res-007','MRN-00007','Evelyn',  'Martinez', '1950-12-01','F','Diabetes type 2 mgmt',   'standard',  1,'2024-02-14T13:30:00Z','2024-02-14T13:30:00Z'),
  ('res-008','MRN-00008','George',  'Thompson', '1936-08-08','M','CHF management',          'skilled',   1,'2024-03-01T10:00:00Z','2024-03-01T10:00:00Z'),
  ('res-009','MRN-00009','Florence','Anderson', '1955-04-17','F','Spinal stenosis',         'standard',  1,'2024-03-10T15:00:00Z','2024-03-10T15:00:00Z'),
  ('res-010','MRN-00010','Arthur',  'Davis',    '1929-10-30','M','Dementia moderate',       'memory',    1,'2024-03-15T09:00:00Z','2024-03-15T09:00:00Z');

-- Active admissions (no discharged_at)
INSERT OR IGNORE INTO admissions (id, resident_id, bed_id, admitted_at, discharged_at, discharge_reason, admitted_by, row_version, created_at, updated_at) VALUES
  ('adm-001','res-001','bed-e101a','2024-01-05T09:00:00Z', NULL,'','usr-frontdesk',1,'2024-01-05T09:00:00Z','2024-01-05T09:00:00Z'),
  ('adm-002','res-002','bed-e102a','2024-01-07T10:30:00Z', NULL,'','usr-frontdesk',1,'2024-01-07T10:30:00Z','2024-01-07T10:30:00Z'),
  ('adm-003','res-003','bed-m01a', '2024-01-10T08:15:00Z', NULL,'','usr-frontdesk',1,'2024-01-10T08:15:00Z','2024-01-10T08:15:00Z'),
  ('adm-004','res-004','bed-e103a','2024-01-12T11:00:00Z', NULL,'','usr-frontdesk',1,'2024-01-12T11:00:00Z','2024-01-12T11:00:00Z'),
  ('adm-005','res-005','bed-w201a','2024-02-01T14:00:00Z', NULL,'','usr-frontdesk',1,'2024-02-01T14:00:00Z','2024-02-01T14:00:00Z'),
  ('adm-006','res-006','bed-m01b', '2024-02-05T09:45:00Z', NULL,'','usr-frontdesk',1,'2024-02-05T09:45:00Z','2024-02-05T09:45:00Z'),
  ('adm-007','res-007','bed-w202a','2024-02-14T13:30:00Z', NULL,'','usr-frontdesk',1,'2024-02-14T13:30:00Z','2024-02-14T13:30:00Z'),
  ('adm-008','res-008','bed-e104a','2024-03-01T10:00:00Z', NULL,'','usr-frontdesk',1,'2024-03-01T10:00:00Z','2024-03-01T10:00:00Z'),
  ('adm-009','res-009','bed-w203a','2024-03-10T15:00:00Z', NULL,'','usr-frontdesk',1,'2024-03-10T15:00:00Z','2024-03-10T15:00:00Z'),
  ('adm-010','res-010','bed-m02a', '2024-03-15T09:00:00Z', NULL,'','usr-frontdesk',1,'2024-03-15T09:00:00Z','2024-03-15T09:00:00Z');

-- Sample occupancy snapshot
INSERT OR IGNORE INTO occupancy_snapshots (id, snapshot_date, total_beds, occupied_beds, available_beds, occupancy_rate, created_at) VALUES
  ('occ-snap-001','2024-04-01', 18, 10, 8, 0.556, '2024-04-01T00:05:00Z');

-- Sample alert events
INSERT OR IGNORE INTO alert_events (id, resident_id, alert_type, severity, message, is_resolved, resolved_by, resolved_at, triggered_at, created_at) VALUES
  ('alert-001','res-001','fall_risk','high',   'Resident assessed as high fall risk post hip repair. Bed alarm required.', 0, NULL, NULL, '2024-04-01T06:00:00Z','2024-04-01T06:00:00Z'),
  ('alert-002','res-004','vitals',  'medium',  'Blood pressure reading elevated: 158/95. Physician notified.',             1, 'usr-nurse','2024-04-01T09:30:00Z','2024-04-01T07:45:00Z','2024-04-01T07:45:00Z'),
  ('alert-003','res-008','medication','medium','Diuretic dosage adjustment pending pharmacy verification.',                0, NULL, NULL,'2024-04-02T08:00:00Z','2024-04-02T08:00:00Z');

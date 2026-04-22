-- Seed 0004: Wards, rooms, beds
-- East Wing: 8 beds (rooms 101–104, 2 beds each)
-- West Wing: 6 beds (rooms 201–203, 2 beds each)
-- Memory Care Unit: 4 beds (rooms M01–M02, 2 beds each)

INSERT OR IGNORE INTO beds (id, ward, room_number, bed_label, bed_type, is_active, row_version, created_at, updated_at) VALUES
  -- East Wing
  ('bed-e101a','East Wing','101','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e101b','East Wing','101','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e102a','East Wing','102','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e102b','East Wing','102','B','bariatric', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e103a','East Wing','103','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e103b','East Wing','103','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e104a','East Wing','104','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-e104b','East Wing','104','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  -- West Wing
  ('bed-w201a','West Wing','201','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-w201b','West Wing','201','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-w202a','West Wing','202','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-w202b','West Wing','202','B','bariatric', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-w203a','West Wing','203','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-w203b','West Wing','203','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  -- Memory Care Unit
  ('bed-m01a', 'Memory Care','M01','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-m01b', 'Memory Care','M01','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-m02a', 'Memory Care','M02','A','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z'),
  ('bed-m02b', 'Memory Care','M02','B','standard', 1, 1,'2024-01-01T00:00:00Z','2024-01-01T00:00:00Z');

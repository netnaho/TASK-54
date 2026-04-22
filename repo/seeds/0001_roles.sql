-- Seed 0001: Roles
INSERT OR IGNORE INTO roles (id, name, description, created_at, updated_at) VALUES
  ('role-admin',               'admin',                'Full system access — facility administrator',                         '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-auditor',             'auditor',              'Read-only access to all records and audit logs',                      '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-nurse',               'nurse',                'Clinical care access: residents, admissions, service orders, alerts', '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-front-desk',          'front_desk',           'Admissions, bed management, and basic resident lookup',              '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-therapist',           'therapist',            'Therapy service orders, exercise library, and execution records',    '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-aide',                'aide',                 'Service execution recording and quality checkpoints',                '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-training-coordinator','training_coordinator', 'Competency exam management and staff training oversight',            '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),
  ('role-finance-clerk',       'finance_clerk',        'Payment recording, settlement shifts, and financial reports',        '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');

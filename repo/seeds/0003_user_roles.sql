-- Seed 0003: User-role mappings
INSERT OR IGNORE INTO user_roles (user_id, role_id, granted_at) VALUES
  ('usr-admin',     'role-admin',                '2024-01-01T00:00:00Z'),
  ('usr-auditor',   'role-auditor',              '2024-01-01T00:00:00Z'),
  ('usr-nurse',     'role-nurse',                '2024-01-01T00:00:00Z'),
  ('usr-frontdesk', 'role-front-desk',           '2024-01-01T00:00:00Z'),
  ('usr-therapist', 'role-therapist',            '2024-01-01T00:00:00Z'),
  ('usr-aide',      'role-aide',                 '2024-01-01T00:00:00Z'),
  ('usr-training',  'role-training-coordinator', '2024-01-01T00:00:00Z'),
  ('usr-finance',   'role-finance-clerk',        '2024-01-01T00:00:00Z');

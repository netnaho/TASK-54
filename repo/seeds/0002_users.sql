-- Seed 0002: Demo users
-- Passwords are bcrypt cost-10 hashes generated at seed-prep time (not at runtime).
-- Default credentials documented in README.md.
--
-- admin@careops.local         → Admin!234567
-- auditor@careops.local       → Auditor!2345
-- nurse@careops.local         → Nurse!234567
-- front.desk@careops.local    → FrontDesk!2345
-- therapist@careops.local     → Therapist!2345
-- aide@careops.local          → Aide!2345678
-- training@careops.local      → Training!2345
-- finance@careops.local       → Finance!2345

INSERT OR IGNORE INTO users (id, email, display_name, password_hash, is_active, row_version, created_at, updated_at) VALUES
  ('usr-admin',
   'admin@careops.local',
   'Alex Administrator',
   '$2a$10$5opNmarRX4zU.3xAPwI7l.bjvJvTo0.T1.SjpeVYqc7JqgcfAall2',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-auditor',
   'auditor@careops.local',
   'Audit Officer',
   '$2b$10$PQEPu5BOFJlQqQ.GIkQyQ.qaEOKzJWnkNdv3MQwus/z59MwiU8C/u',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-nurse',
   'nurse@careops.local',
   'Nina Nurse',
   '$2a$10$dyqi2h2RPDdqKAqxFRbV6.xiI.9tX0ALmUaaNi6CvBUBPTDOJs4ny',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-frontdesk',
   'front.desk@careops.local',
   'Francis Front-Desk',
   '$2b$10$J.OoraRjLgtT0RdtwilJ9.o4WeYVhSj7TJukFmR1EelLOSxEJ02NO',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-therapist',
   'therapist@careops.local',
   'Theo Therapist',
   '$2b$10$4s6NvvSWxbabP3XJMiSQ2eWm3u4IRFEskVQa9TRvy2iAlRCSCc.4m',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-aide',
   'aide@careops.local',
   'Amy Aide',
   '$2a$10$LJ1UbXGMU3J1v/97T3e/NufgdWDWvuk7CTL.WMMmI1EPCxZDPFw3G',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-training',
   'training@careops.local',
   'Tracy Training-Coordinator',
   '$2b$10$a1iANseqfOa.DqcnFedf9.S5/kyfpYFxMi2Kwt5XfolcYU5vo7ryi',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z'),

  ('usr-finance',
   'finance@careops.local',
   'Frank Finance-Clerk',
   '$2b$10$wWWiuplURCVew/IG5paD8upwxdinwmp8WflDTW2YUksMStUnamdJi',
   1, 1, '2024-01-01T00:00:00Z', '2024-01-01T00:00:00Z');

-- Seed 0007: Competency exam templates and sample session

INSERT OR IGNORE INTO competency_exam_templates (id, title, description, role_scope, passing_score, question_bank, is_active, created_by, created_at, updated_at) VALUES
  ('tmpl-001',
   'Basic Infection Control Competency',
   'Annual competency exam covering hand hygiene, PPE use, isolation precautions, and exposure protocols.',
   'aide',
   80,
   '[
     {"id":1,"text":"Hand hygiene must be performed for a minimum of how many seconds?","options":["10 seconds","20 seconds","30 seconds","60 seconds"],"correct":2},
     {"id":2,"text":"Which PPE item is donned first when entering an airborne isolation room?","options":["Gloves","Gown","N95 Respirator","Eye protection"],"correct":3},
     {"id":3,"text":"An exposure to blood or body fluids must be reported within:","options":["4 hours","8 hours","24 hours","72 hours"],"correct":0}
   ]',
   1,'usr-training','2024-01-20T00:00:00Z','2024-01-20T00:00:00Z'),

  ('tmpl-002',
   'Resident Rights and Dignity of Care',
   'Covers resident rights under OBRA, dignity of care standards, and grievance procedures.',
   'nurse',
   85,
   '[
     {"id":1,"text":"Under OBRA, a resident has the right to refuse treatment.","options":["True","False"],"correct":0},
     {"id":2,"text":"Who should be notified first when a resident files a formal grievance?","options":["State surveyor","Facility administrator","Resident''s family","Charge nurse"],"correct":1}
   ]',
   1,'usr-training','2024-01-20T00:00:00Z','2024-01-20T00:00:00Z'),

  ('tmpl-003',
   'Medication Administration Safety',
   'Reviews the 5 Rights of medication administration and common high-alert drug categories.',
   'nurse',
   90,
   '[
     {"id":1,"text":"Which is NOT one of the 5 Rights of medication administration?","options":["Right patient","Right route","Right cost","Right time"],"correct":2},
     {"id":2,"text":"Anticoagulants are classified as high-alert medications.","options":["True","False"],"correct":0},
     {"id":3,"text":"A medication error must be documented within:","options":["1 hour","4 hours","12 hours","24 hours"],"correct":1}
   ]',
   1,'usr-training','2024-01-22T00:00:00Z','2024-01-22T00:00:00Z');

-- Sample scheduled exam session
INSERT OR IGNORE INTO competency_exam_sessions (id, template_id, scheduled_by, scheduled_date, location, status, row_version, created_at, updated_at) VALUES
  ('sess-001','tmpl-001','usr-training','2024-04-10','Conference Room B','scheduled',1,'2024-03-25T00:00:00Z','2024-03-25T00:00:00Z'),
  ('sess-002','tmpl-003','usr-training','2024-04-15','Nursing Station 1','scheduled',1,'2024-03-25T00:00:00Z','2024-03-25T00:00:00Z');

-- Candidates
INSERT OR IGNORE INTO competency_exam_candidates (id, session_id, user_id, score, passed, graded_at, graded_by, notes, created_at) VALUES
  ('cand-001','sess-001','usr-aide',   NULL, NULL, NULL, NULL, '', '2024-03-26T00:00:00Z'),
  ('cand-002','sess-002','usr-nurse',  NULL, NULL, NULL, NULL, '', '2024-03-26T00:00:00Z');

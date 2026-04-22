-- Seed 0006: Exercise library items and tags

INSERT OR IGNORE INTO exercise_tags (id, name, color) VALUES
  ('tag-balance',     'Balance',       '#3B82F6'),
  ('tag-strength',    'Strength',      '#10B981'),
  ('tag-flexibility', 'Flexibility',   '#8B5CF6'),
  ('tag-cardio',      'Cardio',        '#EF4444'),
  ('tag-cognitive',   'Cognitive',     '#F59E0B'),
  ('tag-ot',          'Occupational',  '#EC4899'),
  ('tag-gait',        'Gait Training', '#14B8A6'),
  ('tag-aquatic',     'Aquatic',       '#0EA5E9'),
  ('tag-seated',      'Seated',        '#64748B'),
  ('tag-iadl',        'IADL',          '#A16207');

INSERT OR IGNORE INTO exercise_items (id, name, description, category, difficulty, duration_minutes, instructions, contraindications, is_active, created_by, created_at, updated_at) VALUES
  ('ex-001',
   'Single-Leg Stance',
   'Stand on one leg to improve balance and proprioception.',
   'balance','beginner', 5,
   '1. Stand near a wall for safety. 2. Lift one foot 2-4 inches off the floor. 3. Hold 10–30 seconds. 4. Alternate legs. Repeat 3 sets.',
   'Active lower-limb fracture; severe vertigo.',
   1,'usr-therapist','2024-01-15T00:00:00Z','2024-01-15T00:00:00Z'),

  ('ex-002',
   'Seated Leg Press (Theraband)',
   'Strengthen quadriceps and glutes using a resistance band from a seated position.',
   'strength','beginner', 15,
   '1. Sit in a sturdy chair. 2. Loop band around foot. 3. Extend leg against resistance. 4. Hold 2 sec, return slowly. 3 sets × 12 reps per leg.',
   'Active knee inflammation; DVT.',
   1,'usr-therapist','2024-01-15T00:00:00Z','2024-01-15T00:00:00Z'),

  ('ex-003',
   'Supine Hip Flexor Stretch',
   'Gentle hip flexor stretch to reduce tightness and improve mobility.',
   'flexibility','beginner', 10,
   '1. Lie on back. 2. Bring one knee to chest. 3. Hold 20–30 seconds. 4. Alternate sides. 3 repetitions each.',
   'Recent hip replacement (consult surgeon); lumbar instability.',
   1,'usr-therapist','2024-01-15T00:00:00Z','2024-01-15T00:00:00Z'),

  ('ex-004',
   'Seated Marching',
   'Low-impact cardiovascular activity and hip flexion strengthening from a chair.',
   'cardio','beginner', 10,
   '1. Sit upright in a chair. 2. Alternate lifting knees as if marching. 3. Maintain steady pace for 5–10 minutes.',
   'Severe shortness of breath at rest; unstable angina.',
   1,'usr-therapist','2024-01-15T00:00:00Z','2024-01-15T00:00:00Z'),

  ('ex-005',
   'Word Recall Exercise',
   'Short-term memory training via word list recall.',
   'cognitive','beginner', 15,
   '1. Read a list of 10 words aloud. 2. Wait 2 minutes (distraction activity). 3. Resident recalls as many words as possible. Document score.',
   'Active delirium episode.',
   1,'usr-therapist','2024-01-15T00:00:00Z','2024-01-15T00:00:00Z'),

  ('ex-006',
   'Tandem Walk',
   'Heel-to-toe walking along a straight line for advanced balance and gait training.',
   'balance','intermediate', 10,
   '1. Place a strip of tape on floor (10 feet). 2. Walk heel-to-toe along the line. 3. Arms out for balance assist. 4. 3 passes.',
   'Recent fall within 48 hrs; orthostatic hypotension.',
   1,'usr-therapist','2024-01-16T00:00:00Z','2024-01-16T00:00:00Z'),

  ('ex-007',
   'Dressing & Grooming IADL Practice',
   'Guided retraining of upper-extremity ADL skills: button fastening, zipper, and comb use.',
   'cognitive','beginner', 20,
   '1. Present adaptive equipment. 2. Guide resident through shirt buttoning sequence. 3. Progresses to zipper and hair comb. Document skill level.',
   'Severe hand edema; upper extremity contracture.',
   1,'usr-therapist','2024-01-17T00:00:00Z','2024-01-17T00:00:00Z'),

  ('ex-008',
   'Upper Body Resistance Band Circuit',
   'Multi-set shoulder and arm strengthening using light resistance bands.',
   'strength','intermediate', 20,
   '1. Bicep curl × 15. 2. Overhead press × 12. 3. Lateral raise × 12. 4. Rest 60 sec. Repeat 3 circuits.',
   'Rotator cuff tear (unrepaired); shoulder bursitis flare.',
   1,'usr-therapist','2024-01-18T00:00:00Z','2024-01-18T00:00:00Z');

-- Tag associations
INSERT OR IGNORE INTO exercise_item_tags (exercise_id, tag_id) VALUES
  ('ex-001','tag-balance'),
  ('ex-001','tag-strength'),
  ('ex-002','tag-strength'),
  ('ex-002','tag-seated'),
  ('ex-003','tag-flexibility'),
  ('ex-004','tag-cardio'),
  ('ex-004','tag-seated'),
  ('ex-005','tag-cognitive'),
  ('ex-006','tag-balance'),
  ('ex-006','tag-gait'),
  ('ex-007','tag-ot'),
  ('ex-007','tag-iadl'),
  ('ex-007','tag-cognitive'),
  ('ex-008','tag-strength');

-- Favorites for the therapist user
INSERT OR IGNORE INTO exercise_favorites (user_id, exercise_id, created_at) VALUES
  ('usr-therapist','ex-001','2024-02-01T00:00:00Z'),
  ('usr-therapist','ex-003','2024-02-01T00:00:00Z'),
  ('usr-therapist','ex-007','2024-02-05T00:00:00Z');

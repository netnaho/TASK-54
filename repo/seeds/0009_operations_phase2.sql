-- Seed 0009: Operations phase 2 — richer service delivery data and exercise metadata

-- ── Exercise item enrichment ─────────────────────────────────────────────────
-- Adds body_part, coaching_points, and local media references to seeded exercises.
UPDATE exercise_items SET
    body_part = 'lower body',
    coaching_points = 'Keep feet shoulder-width apart. Shift weight slowly. Do not lock knees. Instructor stands beside, not in front.',
    media_references = '[{"type":"image","filename":"balance-standing-01.jpg","caption":"Starting position"},{"type":"image","filename":"balance-standing-02.jpg","caption":"Weight shift right"}]'
WHERE id = 'ex-001';

UPDATE exercise_items SET
    body_part = 'lower body',
    coaching_points = 'Walk heel-to-toe; arms out for balance. Spotter walks parallel one step behind. Stop immediately if resident reaches for wall.',
    media_references = '[{"type":"image","filename":"tandem-walk-01.jpg","caption":"Tandem walk setup"}]'
WHERE id = 'ex-002';

UPDATE exercise_items SET
    body_part = 'upper body',
    coaching_points = 'Use light resistance band. Exhale on exertion. Rest 30 s between sets. Contraindications must be screened before first session.',
    media_references = '[{"type":"image","filename":"resistance-band-row-01.jpg","caption":"Band row start"},{"type":"video","filename":"resistance-band-demo.mp4","caption":"Full movement"}]'
WHERE id = 'ex-003';

UPDATE exercise_items SET
    body_part = 'lower body',
    coaching_points = 'Chair must have armrests and no wheels. Feet flat on floor. Rise slowly to avoid orthostatic hypotension. Count 3 seconds up, 3 down.',
    media_references = '[{"type":"image","filename":"chair-squat-01.jpg","caption":"Seated position"},{"type":"image","filename":"chair-squat-02.jpg","caption":"Standing position"}]'
WHERE id = 'ex-004';

UPDATE exercise_items SET
    body_part = 'lower body / back',
    coaching_points = 'Perform seated or standing. Keep back straight. Hold each stretch 20–30 s. Never bounce. Monitor for hip pain.',
    media_references = '[{"type":"image","filename":"hip-flex-01.jpg","caption":"Seated hip flexor stretch"}]'
WHERE id = 'ex-005';

UPDATE exercise_items SET
    body_part = 'full body',
    coaching_points = 'Low-impact only. RPE target 11–13. Monitor O2 sat and HR. Stop if resident becomes breathless or HR > 110.',
    media_references = '[{"type":"image","filename":"seated-cardio-01.jpg","caption":"Seated marching"}]'
WHERE id = 'ex-006';

UPDATE exercise_items SET
    body_part = 'cognitive',
    coaching_points = 'Read card slowly, give 10 s to answer. Record errors without feedback. Engage family for context on memory gaps.',
    media_references = '[{"type":"image","filename":"memory-cards-01.jpg","caption":"Sample memory card set"}]'
WHERE id = 'ex-007';

UPDATE exercise_items SET
    body_part = 'cognitive / fine motor',
    coaching_points = 'Allow resident to self-pace. Occupational therapist should observe first session. Document grip strength and finger isolation.',
    media_references = '[{"type":"image","filename":"puzzle-01.jpg","caption":"Large-piece puzzle board"}]'
WHERE id = 'ex-008';

-- ── Additional service orders ─────────────────────────────────────────────────
-- so-005 must be inserted before ser-110 which references it
INSERT OR IGNORE INTO service_orders (id, resident_id, ordered_by, service_type, frequency, instructions, status, start_date, end_date, row_version, created_at, updated_at) VALUES
  ('so-005','res-008','usr-nurse','PT','3x/week','COPD exacerbation recovery — breathing exercises and chest PT.','active','2024-04-01',NULL,1,'2024-04-01T00:00:00Z','2024-04-01T00:00:00Z'),
  ('so-101','res-002','usr-nurse','OT','3x/week','ADL training following right shoulder arthroplasty.','active','2024-01-10',NULL,1,'2024-01-10T00:00:00Z','2024-01-10T00:00:00Z'),
  ('so-102','res-003','usr-nurse','PT','5x/week','Gait retraining and balance work post-fall.','active','2024-01-15',NULL,1,'2024-01-15T00:00:00Z','2024-01-15T00:00:00Z'),
  ('so-103','res-006','usr-nurse','ST','3x/week','Swallowing evaluation and dysphagia diet management.','active','2024-02-10',NULL,1,'2024-02-10T00:00:00Z','2024-02-10T00:00:00Z'),
  ('so-104','res-007','usr-nurse','nursing','daily','Wound care — left heel pressure ulcer stage II.','active','2024-03-01',NULL,1,'2024-03-01T00:00:00Z','2024-03-01T00:00:00Z'),
  ('so-105','res-010','usr-nurse','PT','2x/week','Gentle ROM exercises for advanced dementia resident.','active','2024-03-15',NULL,1,'2024-03-15T00:00:00Z','2024-03-15T00:00:00Z'),
  ('so-106','res-001','usr-nurse','OT','3x/week','Fine motor and grip-strength retraining.','completed','2024-01-08','2024-03-31',1,'2024-01-08T00:00:00Z','2024-04-01T00:00:00Z');

-- ── Execution records with scheduled_start (for timeliness metrics) ───────────
-- on-time: performed_at ≤ scheduled_start + 15 min
-- late:    performed_at >  scheduled_start + 15 min
-- unscheduled: scheduled_start = ''

INSERT OR IGNORE INTO service_execution_records (id, service_order_id, performed_by, scheduled_start, performed_at, duration_minutes, notes, status, created_at) VALUES
  -- on-time (+8 min)
  ('ser-101','so-001','usr-therapist','2024-04-03T09:00:00Z','2024-04-03T09:08:00Z',45,'Gait training with rolling walker 60 ft. Resident stable.','completed','2024-04-03T09:53:00Z'),
  -- on-time (+14 min)
  ('ser-102','so-002','usr-therapist','2024-04-03T10:00:00Z','2024-04-03T10:14:00Z',40,'Shirt donning achieved 70% independence. Progressing well.','completed','2024-04-03T10:54:00Z'),
  -- late (+22 min)
  ('ser-103','so-003','usr-therapist','2024-04-03T14:00:00Z','2024-04-03T14:22:00Z',30,'Resident was at meal service, session delayed. Lumbar stretches completed.','completed','2024-04-03T14:52:00Z'),
  -- on-time (+5 min)
  ('ser-104','so-004','usr-therapist','2024-04-04T09:00:00Z','2024-04-04T09:05:00Z',45,'Knee flexion now 95°. Quad sets and SLR completed.','completed','2024-04-04T09:50:00Z'),
  -- late (+35 min)
  ('ser-105','so-101','usr-therapist','2024-04-04T10:00:00Z','2024-04-04T10:35:00Z',35,'Late start — equipment in use. Shoulder AROM exercises completed.','completed','2024-04-04T11:10:00Z'),
  -- on-time (+12 min)
  ('ser-106','so-102','usr-therapist','2024-04-04T14:00:00Z','2024-04-04T14:12:00Z',45,'Balance board 5 min without support. Significant improvement.','completed','2024-04-04T14:57:00Z'),
  -- unscheduled (no scheduled_start)
  ('ser-107','so-104','usr-nurse',    '','2024-04-05T08:30:00Z',20,'Wound care as-needed. Stage II slightly improved. Dressing changed.','completed','2024-04-05T08:50:00Z'),
  -- on-time (+3 min)
  ('ser-108','so-103','usr-therapist','2024-04-05T11:00:00Z','2024-04-05T11:03:00Z',30,'Swallowing trial with thickened liquids. Choking event — nursing notified.','completed','2024-04-05T11:33:00Z'),
  -- late (+28 min)
  ('ser-109','so-002','usr-therapist','2024-04-06T10:00:00Z','2024-04-06T10:28:00Z',40,'Resident refused initial contact, agreed after 25 min. Dressing skills session.','completed','2024-04-06T11:08:00Z'),
  -- refused (scheduled but refused)
  ('ser-110','so-005','usr-therapist','2024-04-07T09:00:00Z','2024-04-07T09:02:00Z',0,'Resident declined, stated fatigue. Nursing notified. Not counted toward completion.','refused','2024-04-07T09:03:00Z');

-- ── Care quality checkpoints ──────────────────────────────────────────────────
INSERT OR IGNORE INTO care_quality_checkpoints (id, resident_id, checked_by, checkpoint_type, score, notes, checked_at, created_at) VALUES
  ('cqc-001','res-001','usr-nurse',  'pain_assessment',  4,'Resident reports 4/10 pain post-PT. Analgesic administered per PRN order.','2024-04-03T11:00:00Z','2024-04-03T11:00:00Z'),
  ('cqc-002','res-002','usr-nurse',  'mobility_check',   3,'Ambulates with walker supervision. Some hesitation at threshold.','2024-04-03T11:15:00Z','2024-04-03T11:15:00Z'),
  ('cqc-003','res-004','usr-nurse',  'skin_integrity',   5,'No new skin breakdown. Existing area healing well.','2024-04-03T14:00:00Z','2024-04-03T14:00:00Z'),
  ('cqc-004','res-007','usr-nurse',  'wound_assessment', 3,'Heel wound granulating. Slightly larger than last check. Physician notified.','2024-04-04T08:30:00Z','2024-04-04T08:30:00Z'),
  ('cqc-005','res-005','usr-nurse',  'pain_assessment',  2,'Minimal pain post-knee PT. Ice pack applied.','2024-04-04T10:00:00Z','2024-04-04T10:00:00Z'),
  ('cqc-006','res-003','usr-nurse',  'fall_risk_screen',  2,'Fall risk score elevated post-new medication. Bed alarm activated.','2024-04-05T09:00:00Z','2024-04-05T09:00:00Z'),
  ('cqc-007','res-009','usr-therapist','functional_status',4,'Resident transfers sit-to-stand with minimal assist. Improving.','2024-04-05T14:00:00Z','2024-04-05T14:00:00Z'),
  ('cqc-008','res-010','usr-aide',   'engagement_check',  3,'Resident engaged with puzzle activity for 15 min. Some confusion.','2024-04-06T10:00:00Z','2024-04-06T10:00:00Z'),
  ('cqc-009','res-001','usr-therapist','functional_status',5,'Resident ambulated 100 ft independently. Discharge planning initiated.','2024-04-07T09:00:00Z','2024-04-07T09:00:00Z'),
  ('cqc-010','res-006','usr-nurse',  'nutrition_check',   3,'Dysphagia diet compliance improving. Ate 70% of meal.','2024-04-07T12:30:00Z','2024-04-07T12:30:00Z');

-- ── Additional alert events ───────────────────────────────────────────────────
INSERT OR IGNORE INTO alert_events (id, resident_id, alert_type, severity, message, is_resolved, resolved_by, resolved_at, triggered_at, created_at) VALUES
  ('alert-p2-001','res-007','wound_change','high',  'Heel wound showing signs of infection — redness and warmth noted.',0,NULL,NULL,'2024-04-04T08:35:00Z','2024-04-04T08:35:00Z'),
  ('alert-p2-002','res-003','fall_risk',  'medium', 'Resident ambulating unsafely in hallway without walker. Redirected.',0,NULL,NULL,'2024-04-05T09:05:00Z','2024-04-05T09:05:00Z'),
  ('alert-p2-003','res-008','vitals',     'low',    'O2 sat 91% at rest following chest PT. Physician notified.',1,'usr-nurse','2024-04-05T14:00:00Z','2024-04-05T11:40:00Z','2024-04-05T11:40:00Z');

-- ── Occupancy snapshots (trend data) ─────────────────────────────────────────
INSERT OR IGNORE INTO occupancy_snapshots (id, snapshot_date, total_beds, occupied_beds, available_beds, occupancy_rate, created_at) VALUES
  ('occ-002','2024-04-02',18,11,7,0.611,'2024-04-02T00:05:00Z'),
  ('occ-003','2024-04-03',18,11,7,0.611,'2024-04-03T00:05:00Z'),
  ('occ-004','2024-04-04',18,11,7,0.611,'2024-04-04T00:05:00Z'),
  ('occ-005','2024-04-05',18,10,8,0.556,'2024-04-05T00:05:00Z'),
  ('occ-006','2024-04-06',18,10,8,0.556,'2024-04-06T00:05:00Z'),
  ('occ-007','2024-04-07',18,10,8,0.556,'2024-04-07T00:05:00Z');

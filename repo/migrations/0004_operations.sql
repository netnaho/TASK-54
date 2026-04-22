-- Migration 0004: Operations phase additions
-- Adds scheduled_start to execution records (timeliness tracking)
-- Adds body_part, coaching_points, media_references to exercise items

ALTER TABLE service_execution_records ADD COLUMN scheduled_start TEXT NOT NULL DEFAULT '';

ALTER TABLE exercise_items ADD COLUMN body_part TEXT NOT NULL DEFAULT '';
ALTER TABLE exercise_items ADD COLUMN coaching_points TEXT NOT NULL DEFAULT '';
ALTER TABLE exercise_items ADD COLUMN media_references TEXT NOT NULL DEFAULT '[]';

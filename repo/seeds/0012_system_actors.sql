-- Seed 0012: System actor accounts
-- These are non-human system accounts used for automated operations.
-- They are inactive (cannot log in) and have no roles.
-- The 'scheduler' account is the triggered_by actor for automated report runs.
INSERT OR IGNORE INTO users (id, email, display_name, password_hash, is_active, row_version, created_at, updated_at)
VALUES (
    'scheduler',
    'scheduler@system.local',
    'Automated Scheduler',
    '$2a$10$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA',
    0,
    1,
    '2024-01-01T00:00:00Z',
    '2024-01-01T00:00:00Z'
);

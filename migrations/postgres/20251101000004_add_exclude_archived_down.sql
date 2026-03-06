-- Rollback migration: add_exclude_archived
-- Created: 2025-11-01T00:00:04Z

ALTER TABLE repositories DROP COLUMN IF EXISTS archived;
ALTER TABLE connections DROP COLUMN IF EXISTS exclude_archived;

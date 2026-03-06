-- Rollback migration: add_cleanup_archived
-- Created: 2025-11-01T00:00:08Z

ALTER TABLE connections DROP COLUMN IF EXISTS cleanup_archived;

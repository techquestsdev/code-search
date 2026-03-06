-- Rollback migration: add_exclude_archived (MySQL)
-- Created: 2025-11-01T00:00:04Z

ALTER TABLE repositories DROP COLUMN archived;
ALTER TABLE connections DROP COLUMN exclude_archived;

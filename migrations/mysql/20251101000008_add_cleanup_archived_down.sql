-- Rollback migration: add_cleanup_archived (MySQL)
-- Created: 2025-11-01T00:00:08Z

ALTER TABLE connections DROP COLUMN cleanup_archived;

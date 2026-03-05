-- Rollback migration: add_poll_interval
-- Created: 2025-11-01T00:00:02Z

DROP INDEX IF EXISTS idx_repositories_sync_check;
ALTER TABLE repositories DROP COLUMN IF EXISTS poll_interval_seconds;

-- Rollback migration: add_poll_interval (MySQL)
-- Created: 2025-11-01T00:00:02Z

DROP INDEX idx_repositories_sync_check ON repositories;
ALTER TABLE repositories DROP COLUMN poll_interval_seconds;

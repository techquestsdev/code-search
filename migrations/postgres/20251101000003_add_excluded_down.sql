-- Rollback migration: add_excluded
-- Created: 2025-11-01T00:00:03Z

DROP INDEX IF EXISTS idx_repositories_excluded;
ALTER TABLE repositories DROP COLUMN IF EXISTS excluded;

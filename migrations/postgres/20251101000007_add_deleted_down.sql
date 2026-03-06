-- Migration: add_deleted (rollback)
-- Created: 2025-11-01T00:00:07Z

DROP INDEX IF EXISTS idx_repositories_deleted;
ALTER TABLE repositories DROP COLUMN IF EXISTS deleted;

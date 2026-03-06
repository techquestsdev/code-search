-- Migration: add_deleted (rollback)
-- Created: 2025-11-01T00:00:07Z

DROP INDEX idx_repositories_deleted ON repositories;
ALTER TABLE repositories DROP COLUMN deleted;

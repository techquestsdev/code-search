-- Rollback migration: add_excluded (MySQL)
-- Created: 2025-11-01T00:00:03Z

DROP INDEX idx_repositories_excluded ON repositories;
ALTER TABLE repositories DROP COLUMN excluded;

-- Rollback migration: init
-- Created: 2025-11-01T00:00:01Z

DROP TRIGGER IF EXISTS update_replace_operations_updated_at ON replace_operations;
DROP TRIGGER IF EXISTS update_jobs_updated_at ON jobs;
DROP TRIGGER IF EXISTS update_repositories_updated_at ON repositories;
DROP TRIGGER IF EXISTS update_connections_updated_at ON connections;
DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS replace_operations;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS repositories;
DROP TABLE IF EXISTS connections;

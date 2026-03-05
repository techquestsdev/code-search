-- Migration: add_composite_indexes (down)
-- Rollback composite indexes on repositories table

DROP INDEX IF EXISTS idx_repos_pending_jobs;
DROP INDEX IF EXISTS idx_repos_status_excluded;
DROP INDEX IF EXISTS idx_repos_stale_check;
DROP INDEX IF EXISTS idx_repos_connection_status;

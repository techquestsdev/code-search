-- Migration: add_composite_indexes (down/MySQL)
-- Rollback composite indexes on repositories table

DROP INDEX idx_repos_pending_jobs ON repositories;
DROP INDEX idx_repos_status_excluded ON repositories;
DROP INDEX idx_repos_stale_check ON repositories;
DROP INDEX idx_repos_connection_status ON repositories;

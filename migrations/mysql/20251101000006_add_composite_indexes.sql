-- Migration: add_composite_indexes (MySQL)
-- Created: 2025-11-01T00:00:06Z
-- Purpose: Add composite indexes to improve query performance on repositories table

-- Index for pending job queries: WHERE index_status = 'pending' AND excluded = false ORDER BY created_at
-- Used by: GetPendingIndexJobs, ClaimPendingIndexJob
-- Note: MySQL doesn't support partial indexes, so we use a full composite index
CREATE INDEX idx_repos_pending_jobs ON repositories(index_status, excluded, created_at);

-- Index for status-based queries: WHERE index_status = X AND excluded = false
-- Used by: GetRepositoriesByStatus, ReindexAll, GetStats
CREATE INDEX idx_repos_status_excluded ON repositories(index_status, excluded);

-- Index for stale repo queries: WHERE index_status = 'indexed' AND last_indexed < X
-- Used by: TriggerSyncStaleRepos, GetRepoStats
CREATE INDEX idx_repos_stale_check ON repositories(index_status, last_indexed);

-- Index for connection-scoped queries with filtering
-- Used by: ListRepositories, ReindexConnection
CREATE INDEX idx_repos_connection_status ON repositories(connection_id, excluded, index_status);

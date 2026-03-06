-- Migration: add_poll_interval (MySQL)
-- Created: 2025-11-01T00:00:02Z

-- Add poll_interval_seconds column to repositories table
-- This allows per-repository custom polling intervals

ALTER TABLE repositories
ADD COLUMN poll_interval_seconds INTEGER DEFAULT NULL;

-- Add index for efficient querying of repos needing sync
CREATE INDEX idx_repositories_sync_check
ON repositories(index_status, last_indexed);

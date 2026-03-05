-- Migration: add_poll_interval
-- Created: 2025-11-01T00:00:02Z

-- Add poll_interval_seconds column to repositories table
-- This allows per-repository custom polling intervals

ALTER TABLE repositories
ADD COLUMN IF NOT EXISTS poll_interval_seconds INTEGER DEFAULT NULL;

-- Add comment explaining the column
COMMENT ON COLUMN repositories.poll_interval_seconds IS
    'Custom polling interval in seconds. NULL means use system default.';

-- Add index for efficient querying of repos needing sync
CREATE INDEX IF NOT EXISTS idx_repositories_sync_check
ON repositories(index_status, last_indexed)
WHERE index_status IN ('indexed', 'pending');

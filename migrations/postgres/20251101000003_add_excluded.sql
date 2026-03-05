-- Migration: add_excluded
-- Created: 2025-11-01T00:00:03Z

-- Add excluded column to repositories for soft delete functionality
-- When excluded = true, the repo is skipped during sync and removed from index

ALTER TABLE repositories ADD COLUMN IF NOT EXISTS excluded BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for efficient filtering
CREATE INDEX IF NOT EXISTS idx_repositories_excluded ON repositories(excluded) WHERE excluded = false;

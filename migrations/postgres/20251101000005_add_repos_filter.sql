-- Migration: add_repos_filter
-- Created: 2025-12-05
-- Description: Add repos column to connections for specifying individual repos to sync

-- Add repos column as a text array to store specific repo names/patterns
ALTER TABLE connections ADD COLUMN IF NOT EXISTS repos TEXT[] DEFAULT '{}';

-- Comment explaining the column
COMMENT ON COLUMN connections.repos IS 'Specific repos to sync. If empty, syncs all accessible repos from the code host.';

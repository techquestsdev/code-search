-- Migration: add_repos_filter
-- Created: 2025-12-05
-- Description: Add repos column to connections for specifying individual repos to sync

-- Add repos column as JSON text to store specific repo names/patterns
ALTER TABLE connections ADD COLUMN repos TEXT DEFAULT '';

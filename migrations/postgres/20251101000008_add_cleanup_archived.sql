-- Migration: add_cleanup_archived
-- Created: 2025-11-01T00:00:08Z

-- Add cleanup_archived column to connections table
-- When true, auto-cleanup Zoekt index shards for repos that become archived

ALTER TABLE connections ADD COLUMN IF NOT EXISTS cleanup_archived BOOLEAN NOT NULL DEFAULT FALSE;

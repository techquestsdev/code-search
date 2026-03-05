-- Migration: add_exclude_archived
-- Created: 2025-11-01T00:00:04Z

-- Add exclude_archived column to connections table
-- When true, archived repositories from this connection will be excluded from sync

ALTER TABLE connections ADD COLUMN IF NOT EXISTS exclude_archived BOOLEAN NOT NULL DEFAULT FALSE;

-- Add archived column to repositories table to track archive status from code host
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS archived BOOLEAN NOT NULL DEFAULT FALSE;

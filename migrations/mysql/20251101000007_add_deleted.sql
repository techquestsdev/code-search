-- Migration: add_deleted
-- Created: 2025-11-01T00:00:07Z

-- Add deleted column to repositories for permanent exclusion
-- When deleted = true, the repo is permanently excluded and won't be re-added on sync
-- Unlike 'excluded', deleted repos are meant to never come back unless explicitly restored

ALTER TABLE repositories ADD COLUMN deleted BOOLEAN NOT NULL DEFAULT FALSE;

-- Index for efficient filtering
CREATE INDEX idx_repositories_deleted ON repositories(deleted);

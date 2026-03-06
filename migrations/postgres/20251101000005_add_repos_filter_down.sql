-- Migration: add_repos_filter (down)
-- Rollback: Remove repos column from connections

ALTER TABLE connections DROP COLUMN IF EXISTS repos;

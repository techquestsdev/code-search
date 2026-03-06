-- Migration: init_down (MySQL)
-- Rollback the init migration

DROP TABLE IF EXISTS replace_operations;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS repositories;
DROP TABLE IF EXISTS connections;

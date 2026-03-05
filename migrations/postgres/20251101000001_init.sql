-- Migration: init
-- Created: 2025-11-01T00:00:01Z

-- Connections table (code host configurations)
CREATE TABLE IF NOT EXISTS connections (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,  -- github, gitlab, gitea, bitbucket
    url VARCHAR(1000) NOT NULL,
    token VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Repositories table
CREATE TABLE IF NOT EXISTS repositories (
    id BIGSERIAL PRIMARY KEY,
    connection_id BIGINT REFERENCES connections(id) ON DELETE CASCADE,
    name VARCHAR(500) NOT NULL,
    clone_url VARCHAR(1000) NOT NULL,
    default_branch VARCHAR(255) DEFAULT 'main',
    branches TEXT[] DEFAULT '{}',
    last_indexed TIMESTAMPTZ,
    index_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(connection_id, name)
);

-- Create index for status queries
CREATE INDEX IF NOT EXISTS idx_repositories_index_status ON repositories(index_status);
CREATE INDEX IF NOT EXISTS idx_repositories_last_indexed ON repositories(last_indexed);
CREATE INDEX IF NOT EXISTS idx_repositories_connection_id ON repositories(connection_id);

-- Jobs table (background jobs)
CREATE TABLE IF NOT EXISTS jobs (
    id BIGSERIAL PRIMARY KEY,
    type VARCHAR(50) NOT NULL,  -- index, sync, replace
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    payload JSONB NOT NULL DEFAULT '{}',
    result JSONB,
    error TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_jobs_type ON jobs(type);

-- Replace operations table
CREATE TABLE IF NOT EXISTS replace_operations (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT REFERENCES jobs(id) ON DELETE CASCADE,
    old_pattern TEXT NOT NULL,
    new_pattern TEXT NOT NULL,
    is_regex BOOLEAN DEFAULT FALSE,
    filters JSONB DEFAULT '{}',
    create_mr BOOLEAN DEFAULT FALSE,
    mr_config JSONB DEFAULT '{}',
    total_matches INTEGER DEFAULT 0,
    total_repos INTEGER DEFAULT 0,
    completed_repos INTEGER DEFAULT 0,
    created_mrs TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_replace_operations_job_id ON replace_operations(job_id);

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Triggers to auto-update updated_at (use IF NOT EXISTS pattern via DO block)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_connections_updated_at') THEN
        CREATE TRIGGER update_connections_updated_at
            BEFORE UPDATE ON connections
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_repositories_updated_at') THEN
        CREATE TRIGGER update_repositories_updated_at
            BEFORE UPDATE ON repositories
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_jobs_updated_at') THEN
        CREATE TRIGGER update_jobs_updated_at
            BEFORE UPDATE ON jobs
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'update_replace_operations_updated_at') THEN
        CREATE TRIGGER update_replace_operations_updated_at
            BEFORE UPDATE ON replace_operations
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

-- Migration: init (MySQL)
-- Created: 2025-11-01T00:00:01Z

-- Connections table (code host configurations)
CREATE TABLE IF NOT EXISTS connections (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    type VARCHAR(50) NOT NULL,  -- github, gitlab, gitea, bitbucket
    url VARCHAR(1000) NOT NULL,
    token VARCHAR(500) NOT NULL DEFAULT '',
    exclude_archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Repositories table
CREATE TABLE IF NOT EXISTS repositories (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    connection_id BIGINT,
    name VARCHAR(500) NOT NULL,
    clone_url VARCHAR(1000) NOT NULL,
    default_branch VARCHAR(255) DEFAULT 'main',
    branches JSON DEFAULT NULL,
    last_indexed TIMESTAMP NULL DEFAULT NULL,
    index_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    excluded BOOLEAN NOT NULL DEFAULT FALSE,
    archived BOOLEAN NOT NULL DEFAULT FALSE,
    poll_interval_seconds INT NULL DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    UNIQUE KEY idx_repo_conn_name (connection_id, name),
    CONSTRAINT fk_repo_connection FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- Create indexes for status queries
CREATE INDEX idx_repositories_index_status ON repositories(index_status);
CREATE INDEX idx_repositories_last_indexed ON repositories(last_indexed);
CREATE INDEX idx_repositories_connection_id ON repositories(connection_id);

-- Jobs table (background jobs)
CREATE TABLE IF NOT EXISTS jobs (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    type VARCHAR(50) NOT NULL,  -- index, sync, replace, cleanup
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    payload JSON NOT NULL,
    result JSON DEFAULT NULL,
    error TEXT DEFAULT NULL,
    started_at TIMESTAMP NULL DEFAULT NULL,
    completed_at TIMESTAMP NULL DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_type ON jobs(type);

-- Replace operations table
CREATE TABLE IF NOT EXISTS replace_operations (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    job_id BIGINT,
    old_pattern TEXT NOT NULL,
    new_pattern TEXT NOT NULL,
    is_regex BOOLEAN DEFAULT FALSE,
    filters JSON DEFAULT NULL,
    create_mr BOOLEAN DEFAULT FALSE,
    mr_config JSON DEFAULT NULL,
    total_matches INT DEFAULT 0,
    total_repos INT DEFAULT 0,
    completed_repos INT DEFAULT 0,
    created_mrs JSON DEFAULT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    CONSTRAINT fk_replace_job FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE INDEX idx_replace_operations_job_id ON replace_operations(job_id);

-- Note: MySQL uses ON UPDATE CURRENT_TIMESTAMP instead of triggers for auto-updating updated_at

-- Use an ENUM for strict status control
CREATE TYPE job_status AS ENUM (
    'PENDING', 
    'RUNNING', 
    'COMPLETED', 
    'FAILED', 
    'RETRYING'
);

CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(255) NOT NULL,
    
    -- JSONB is critical here. It allows clients to send any structured 
    -- data without us needing to alter the schema for every new job type.
    payload JSONB NOT NULL,
    
    status job_status NOT NULL DEFAULT 'PENDING',
    
    -- Nullable fields for when the job finishes
    result JSONB,
    error_details TEXT,
    
    -- Retry mechanism tracking
    max_retries INTEGER NOT NULL DEFAULT 3,
    retries_attempted INTEGER NOT NULL DEFAULT 0,
    
    -- For scheduled jobs (e.g., "send email in 2 hours")
    run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexing: 
-- 1. We need to quickly find jobs that are PENDING and ready to run.
-- This is for our "Sweeper" process to pick up dropped jobs.
CREATE INDEX idx_jobs_status_run_at ON jobs (status, run_at);

-- 2. We often query jobs by their type to monitor queue health.
CREATE INDEX idx_jobs_type ON jobs (type);
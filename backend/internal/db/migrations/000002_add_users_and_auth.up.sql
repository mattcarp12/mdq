CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    -- Store a hash. Never store plain text.
    password_hash VARCHAR(255) NOT NULL, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Alter our existing jobs table
ALTER TABLE jobs 
ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;

-- SOTA Indexing: We will frequently query "Show me all jobs for User X"
CREATE INDEX idx_jobs_user_id ON jobs(user_id);

-- Insert our hardcoded test user. 
-- The hash below is the bcrypt hash for the password: "password123"
INSERT INTO users (id, email, password_hash) 
VALUES (
    '11111111-1111-1111-1111-111111111111', 
    'test@example.com', 
    '$2a$12$pkkrDL25yEk/4YT5XAthjup2Wk.kCwAKW7kiX.lX/6NtB.ZsY7gzO'
) ON CONFLICT DO NOTHING;
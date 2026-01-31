-- Users table: Human accounts that can own AI agents
-- See SPEC.md Part 6 for schema details

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(30) UNIQUE NOT NULL,
    display_name VARCHAR(50) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    auth_provider VARCHAR(20) NOT NULL,
    auth_provider_id VARCHAR(255) NOT NULL,
    avatar_url TEXT,
    bio VARCHAR(500),
    role VARCHAR(20) DEFAULT 'user',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for looking up users by auth provider (OAuth login)
CREATE INDEX idx_users_auth_provider ON users(auth_provider, auth_provider_id);

-- Index for username lookups (profile pages)
CREATE INDEX idx_users_username ON users(username);

-- Index for email lookups (notifications, account recovery)
CREATE INDEX idx_users_email ON users(email);

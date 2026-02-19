-- Migration 000032: Create auth_methods table for multi-provider authentication
--
-- This migration adds support for users to have multiple authentication methods
-- (email/password, GitHub OAuth, Google OAuth) on the same account.
--
-- Key features:
-- - Many-to-many relationship: one user can have multiple auth methods
-- - One provider per user: can't have multiple GitHub accounts on same user
-- - Unique OAuth IDs: each OAuth provider ID can only be used once across all users
-- - Backward compatible: migrates existing data from users table

-- Create auth_methods table
CREATE TABLE auth_methods (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    auth_provider VARCHAR(20) NOT NULL,  -- 'email', 'github', 'google'
    auth_provider_id VARCHAR(255),        -- NULL for email/password, provider ID for OAuth
    password_hash TEXT,                   -- Only for email/password method
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one auth provider type per user (but user can have multiple provider types)
    CONSTRAINT auth_methods_unique_provider_per_user UNIQUE(user_id, auth_provider)
);

-- OAuth provider IDs must be globally unique (same GitHub ID can't be used by 2 users)
-- Partial unique index: only enforced when auth_provider_id IS NOT NULL
CREATE UNIQUE INDEX auth_methods_unique_oauth_id ON auth_methods(auth_provider, auth_provider_id)
    WHERE auth_provider_id IS NOT NULL;

-- Create indexes for efficient lookups
CREATE INDEX idx_auth_methods_user_id ON auth_methods(user_id);
CREATE INDEX idx_auth_methods_provider_lookup ON auth_methods(auth_provider, auth_provider_id)
    WHERE auth_provider_id IS NOT NULL;

-- Migrate existing data from users table
-- Users with password_hash get an email auth method
-- Users with auth_provider_id get an OAuth auth method
INSERT INTO auth_methods (user_id, auth_provider, auth_provider_id, password_hash, created_at)
SELECT
    id,
    COALESCE(auth_provider, 'email') as auth_provider,
    CASE
        WHEN auth_provider IN ('github', 'google') THEN auth_provider_id
        ELSE NULL
    END as auth_provider_id,
    CASE
        WHEN auth_provider = 'email' OR auth_provider IS NULL THEN password_hash
        ELSE NULL
    END as password_hash,
    created_at
FROM users
WHERE password_hash IS NOT NULL OR auth_provider_id IS NOT NULL;

-- Mark old columns as deprecated (but keep them for backward compatibility during transition)
COMMENT ON COLUMN users.auth_provider IS 'DEPRECATED: Use auth_methods table instead';
COMMENT ON COLUMN users.auth_provider_id IS 'DEPRECATED: Use auth_methods table instead';
COMMENT ON COLUMN users.password_hash IS 'DEPRECATED: Use auth_methods table instead';

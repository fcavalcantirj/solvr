-- Refresh tokens table
-- Stores hashed refresh tokens for user authentication
-- Per SPEC.md Part 5.2: Refresh token is opaque, 7 days expiry, stored in httpOnly cookies

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for user lookup (find all tokens for a user, e.g., for logout all sessions)
CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- Index for token lookup (authenticate by token hash)
CREATE INDEX idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);

-- Index for cleanup of expired tokens
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

-- Unique constraint on token_hash to prevent duplicates
CREATE UNIQUE INDEX idx_refresh_tokens_token_hash_unique ON refresh_tokens(token_hash);

-- Comment on table
COMMENT ON TABLE refresh_tokens IS 'Refresh tokens for user authentication sessions';
COMMENT ON COLUMN refresh_tokens.id IS 'Unique identifier for the token record';
COMMENT ON COLUMN refresh_tokens.user_id IS 'User this refresh token belongs to';
COMMENT ON COLUMN refresh_tokens.token_hash IS 'Hashed refresh token (never store plain tokens)';
COMMENT ON COLUMN refresh_tokens.expires_at IS 'Token expiration time (default 7 days from creation)';
COMMENT ON COLUMN refresh_tokens.created_at IS 'When the token was created';

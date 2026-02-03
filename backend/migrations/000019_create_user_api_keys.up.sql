-- User API Keys table
-- Per SPEC.md Part 5.2 and prd-v2.json API-KEYS requirements
-- Users can have multiple API keys with different names for different purposes

CREATE TABLE user_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for quick lookup during auth (by user)
CREATE INDEX idx_user_api_keys_user_id ON user_api_keys(user_id);

-- Index for finding active keys for a user
CREATE INDEX idx_user_api_keys_active ON user_api_keys(user_id) WHERE revoked_at IS NULL;

-- Comment on columns
COMMENT ON TABLE user_api_keys IS 'API keys for human users (separate from agent API keys)';
COMMENT ON COLUMN user_api_keys.name IS 'User-provided name for the key (e.g., Production, Development)';
COMMENT ON COLUMN user_api_keys.key_hash IS 'Bcrypt hash of the API key - never store plaintext';
COMMENT ON COLUMN user_api_keys.last_used_at IS 'Tracks when the key was last used for security audit';
COMMENT ON COLUMN user_api_keys.revoked_at IS 'NULL means active, set means revoked (soft delete)';

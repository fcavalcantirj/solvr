-- Config table
-- Stores key-value configuration settings with JSONB values
-- Per SPEC.md Part 6: Config table for application settings

CREATE TABLE config (
    key VARCHAR(100) PRIMARY KEY,
    value JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for faster lookups by key prefix (e.g., 'app.', 'feature.')
CREATE INDEX idx_config_key ON config(key);

-- Comment on table
COMMENT ON TABLE config IS 'Application configuration key-value store';
COMMENT ON COLUMN config.key IS 'Unique configuration key (max 100 chars)';
COMMENT ON COLUMN config.value IS 'Configuration value stored as JSONB';
COMMENT ON COLUMN config.created_at IS 'When the config entry was created';
COMMENT ON COLUMN config.updated_at IS 'When the config entry was last updated';

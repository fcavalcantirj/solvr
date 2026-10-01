CREATE INDEX IF NOT EXISTS idx_rate_limit_config_key ON rate_limit_config (key);
CREATE INDEX IF NOT EXISTS idx_config_key ON config (key);
CREATE INDEX IF NOT EXISTS idx_tags_name ON tags (name);
CREATE INDEX IF NOT EXISTS idx_auth_methods_provider_lookup ON auth_methods (auth_provider, auth_provider_id)
    WHERE auth_provider_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_room_entries_room_seq ON room_entries (room_id, sequence);

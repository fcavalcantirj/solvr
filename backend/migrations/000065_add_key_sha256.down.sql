DROP INDEX IF EXISTS idx_agents_key_sha256;
ALTER TABLE agents DROP COLUMN IF EXISTS key_sha256;

DROP INDEX IF EXISTS idx_user_api_keys_sha256;
ALTER TABLE user_api_keys DROP COLUMN IF EXISTS key_sha256;

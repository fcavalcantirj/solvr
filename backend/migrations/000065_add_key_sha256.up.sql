-- Add SHA256 hash column for O(1) API key lookup.
-- The existing bcrypt-only approach requires O(n) sequential comparisons
-- (~50ms each), causing 12-16s auth delays at 287+ active keys.
-- SHA256 is deterministic and indexable for instant lookup;
-- bcrypt is kept for security verification after the SHA256 narrows to 1 row.

-- User API keys: add key_sha256 (nullable for lazy backfill of existing keys)
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS key_sha256 VARCHAR(64);

-- Unique index for O(1) lookup (only non-null values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_api_keys_sha256
    ON user_api_keys(key_sha256) WHERE key_sha256 IS NOT NULL;

-- Agents: add key_sha256 (nullable for lazy backfill)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_sha256 VARCHAR(64);

-- Unique index for O(1) lookup (only non-null values)
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_key_sha256
    ON agents(key_sha256) WHERE key_sha256 IS NOT NULL;

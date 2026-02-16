-- Rollback migration 000032: Remove auth_methods table
--
-- This restores the single-provider authentication system.
-- WARNING: This will lose data if users have multiple auth methods linked.

-- Drop the auth_methods table (CASCADE will remove dependent objects)
DROP TABLE IF EXISTS auth_methods CASCADE;

-- Remove deprecation comments from users table
COMMENT ON COLUMN users.auth_provider IS NULL;
COMMENT ON COLUMN users.auth_provider_id IS NULL;
COMMENT ON COLUMN users.password_hash IS NULL;

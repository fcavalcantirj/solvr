-- Migration: Add password_hash column and make OAuth fields nullable
-- This enables email/password authentication as an alternative to OAuth.
--
-- Changes:
-- 1. Add password_hash column (nullable) - stores bcrypt hashes for email/password users
-- 2. Make auth_provider nullable - email/password users won't have OAuth provider
-- 3. Make auth_provider_id nullable - email/password users won't have OAuth ID
--
-- Backward compatibility: All existing OAuth users remain functional.
-- New capability: Users can now sign up with email/password instead of OAuth.

-- Add password_hash column (nullable, will be NULL for OAuth-only users)
ALTER TABLE users ADD COLUMN password_hash VARCHAR(255);

-- Make OAuth fields nullable to support email/password authentication
ALTER TABLE users ALTER COLUMN auth_provider DROP NOT NULL;
ALTER TABLE users ALTER COLUMN auth_provider_id DROP NOT NULL;

-- Add comment for clarity
COMMENT ON COLUMN users.password_hash IS 'Bcrypt hash of user password. NULL for OAuth-only users. Never exposed via API.';
COMMENT ON COLUMN users.auth_provider IS 'OAuth provider (github, google) or "email" for email/password auth. Can be NULL if user has not set up this auth method yet.';
COMMENT ON COLUMN users.auth_provider_id IS 'ID from OAuth provider. NULL for email/password users.';

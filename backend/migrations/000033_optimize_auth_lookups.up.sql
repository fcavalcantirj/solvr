-- Optimize authentication lookups
-- Migration 000033: Add index for auth method last_used tracking

-- Index for auth method last_used_at tracking (useful for security analytics)
-- Helps identify dormant auth methods and track recent login patterns per user
-- Example queries:
--   - Find users who haven't logged in for 90+ days
--   - Identify auth methods that should be removed due to inactivity
--   - Security audit of recent authentication activity
CREATE INDEX IF NOT EXISTS idx_auth_methods_last_used
ON auth_methods(last_used_at DESC);

-- Note: The following indexes already exist from previous migrations:
-- - idx_users_email (migration 000001): Fast email lookups for account linking
-- - idx_auth_methods_user_id (migration 000032): Find all auth methods for a user
-- - idx_auth_methods_provider_lookup (migration 000032): OAuth provider ID lookups

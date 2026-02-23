-- Add revoked_at to refresh_tokens to support token revocation (logout, security events).
ALTER TABLE refresh_tokens ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ;

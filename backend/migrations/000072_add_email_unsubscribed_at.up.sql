-- Add email_unsubscribed_at to users table for tracking email opt-out.
-- When set (non-NULL), the user is excluded from broadcast emails.
-- Users can unsubscribe via a one-click HMAC-signed link in email headers.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_unsubscribed_at TIMESTAMPTZ;

-- Add admin_type column to audit_log to support agent admins in addition to human admins.
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS admin_type VARCHAR(50);

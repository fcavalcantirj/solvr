DROP TABLE IF EXISTS email_events;
DROP TABLE IF EXISTS email_messages;
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_suppressed_reason_check;
ALTER TABLE users DROP COLUMN IF EXISTS email_suppressed_reason;
ALTER TABLE users DROP COLUMN IF EXISTS email_suppressed_at;

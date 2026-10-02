DROP INDEX IF EXISTS idx_notifications_reply_id;
DROP INDEX IF EXISTS idx_notifications_post_id;
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_reply_of_post_fkey,
    DROP CONSTRAINT IF EXISTS notifications_schema_version_check,
    DROP COLUMN IF EXISTS reply_id,
    DROP COLUMN IF EXISTS post_id,
    DROP COLUMN IF EXISTS schema_version;

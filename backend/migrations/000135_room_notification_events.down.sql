-- The room events leave the contract. Their notifications are kept as rows outside it
-- (version 0, no subject; the type stays as a name no producer writes), their queued
-- deliveries are dropped, and no webhook subscribes to them any more.
DELETE FROM webhook_deliveries WHERE schema_version = 2;
ALTER TABLE webhook_deliveries DROP CONSTRAINT IF EXISTS webhook_deliveries_schema_version_check;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_schema_version_check CHECK (schema_version = 1);

UPDATE notifications SET schema_version = 0, room_id = NULL WHERE schema_version = 2;
DROP INDEX IF EXISTS idx_notifications_room_id;
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_room_subject_version_check,
    DROP CONSTRAINT IF EXISTS notifications_schema_version_check,
    DROP COLUMN IF EXISTS room_id;
ALTER TABLE notifications ADD CONSTRAINT notifications_schema_version_check CHECK (schema_version IN (0, 1));

UPDATE webhooks SET events = array_remove(array_remove(events, 'room.member_added'), 'room.member_removed')
WHERE events && ARRAY['room.member_added', 'room.member_removed']::text[];
COMMENT ON COLUMN webhooks.events IS 'Notification event types of schema version 1: post.approved, post.rejected, reply.removed, reply.flagged, blog_post_rejected';

-- The version 3 room events leave the contract: their queued deliveries are dropped, their
-- notifications kept as rows outside it (version 0, no subject), no webhook subscribes to
-- them, and the opt-in and pause state are removed.
ALTER TABLE agents DROP COLUMN IF EXISTS room_notifications_paused_at;
ALTER TABLE users DROP COLUMN IF EXISTS room_notifications_paused_at;
DROP TABLE IF EXISTS room_notification_subscriptions;

DELETE FROM webhook_deliveries WHERE schema_version = 3;
ALTER TABLE webhook_deliveries DROP CONSTRAINT IF EXISTS webhook_deliveries_schema_version_check;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_schema_version_check CHECK (schema_version IN (1, 2));

DROP INDEX IF EXISTS uq_notifications_entry_agent;
DROP INDEX IF EXISTS uq_notifications_entry_user;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_entry_subject_version_check;
UPDATE notifications SET schema_version = 0, room_id = NULL, entry_id = NULL WHERE schema_version = 3;
ALTER TABLE notifications DROP COLUMN IF EXISTS entry_id;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_room_subject_version_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_room_subject_version_check CHECK (room_id IS NULL OR schema_version = 2);
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_schema_version_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_schema_version_check CHECK (schema_version IN (0, 1, 2));

UPDATE webhooks SET events = array_remove(array_remove(events, 'room.reply'), 'room.review_requested')
WHERE events && ARRAY['room.reply', 'room.review_requested']::text[];
COMMENT ON COLUMN webhooks.events IS 'Notification event types of the contract: post.approved, post.rejected, reply.removed, reply.flagged, blog_post_rejected (version 1); room.member_added, room.member_removed (version 2)';

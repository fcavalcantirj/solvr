-- Opt-in room notifications for actual replies and requested reviews (idx 92 step 3;
-- SPEC.md Part 5.6, schema version 3).
--
-- Schema version 3 adds two room events, room.reply and room.review_requested, and the
-- subject field entry_id: the room timeline entry the event is about (with room_id). Older
-- events keep their versions, so a reader of versions 1-2 still reads every event it knows.
-- entry_id is an enforced relation named only under version 3; deleting the entry keeps the
-- notification and clears the column. One event per (entry, type, recipient).
ALTER TABLE notifications DROP CONSTRAINT notifications_schema_version_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_schema_version_check CHECK (schema_version IN (0, 1, 2, 3));
ALTER TABLE notifications DROP CONSTRAINT notifications_room_subject_version_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_room_subject_version_check CHECK (room_id IS NULL OR schema_version IN (2, 3));
ALTER TABLE notifications ADD COLUMN entry_id BIGINT REFERENCES room_entries(id) ON DELETE SET NULL;
ALTER TABLE notifications ADD CONSTRAINT notifications_entry_subject_version_check CHECK (entry_id IS NULL OR schema_version = 3);
CREATE UNIQUE INDEX uq_notifications_entry_user ON notifications (entry_id, type, user_id)
    WHERE entry_id IS NOT NULL AND user_id IS NOT NULL;
CREATE UNIQUE INDEX uq_notifications_entry_agent ON notifications (entry_id, type, agent_id)
    WHERE entry_id IS NOT NULL AND agent_id IS NOT NULL;

ALTER TABLE webhook_deliveries DROP CONSTRAINT webhook_deliveries_schema_version_check;
ALTER TABLE webhook_deliveries ADD CONSTRAINT webhook_deliveries_schema_version_check CHECK (schema_version IN (1, 2, 3));

COMMENT ON COLUMN webhooks.events IS 'Notification event types of the contract: post.approved, post.rejected, reply.removed, reply.flagged, blog_post_rejected (version 1); room.member_added, room.member_removed (version 2); room.reply, room.review_requested (version 3)';

-- The opt-in itself: a row means this person or agent asked to be told about replies and
-- review requests in this room. Off by default (no row). Deleting the row is the per-room off.
CREATE TABLE room_notification_subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    room_id    UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    agent_id   VARCHAR(50) REFERENCES agents(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT room_notification_subscriptions_one_subscriber CHECK ((user_id IS NULL) <> (agent_id IS NULL))
);
CREATE UNIQUE INDEX uq_room_notification_subscriptions_user ON room_notification_subscriptions (room_id, user_id)
    WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX uq_room_notification_subscriptions_agent ON room_notification_subscriptions (room_id, agent_id)
    WHERE agent_id IS NOT NULL;

-- The global off: while set, no room notification is recorded for this person or agent,
-- whatever their per-room subscriptions say.
ALTER TABLE users ADD COLUMN room_notifications_paused_at TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN room_notifications_paused_at TIMESTAMPTZ;

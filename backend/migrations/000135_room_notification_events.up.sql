-- Room events in the notification event contract (task "Keep SDKs, CLI, MCP, skills, and
-- webhooks consistent with the redesigned product", step 4; SPEC.md Part 5.6 and 12.3).
--
-- Schema version 2 adds the room membership events, room.member_added and
-- room.member_removed, and their subject: room_id, the canonical room. New event types and
-- subject fields come under a new version; the version 1 events keep version 1, so a reader
-- of version 1 still reads every event it knows.
--
-- room_id is an enforced relation, set only under version 2. Deleting the room keeps the
-- notification and clears the column, as a deleted post or reply does.
ALTER TABLE notifications DROP CONSTRAINT notifications_schema_version_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_schema_version_check CHECK (schema_version IN (0, 1, 2)),
    ADD COLUMN room_id UUID REFERENCES rooms(id) ON DELETE SET NULL;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_room_subject_version_check CHECK (room_id IS NULL OR schema_version = 2);

-- The ON DELETE action looks rows up by room.
CREATE INDEX idx_notifications_room_id ON notifications(room_id) WHERE room_id IS NOT NULL;

-- A delivery carries the version its event was written under.
ALTER TABLE webhook_deliveries DROP CONSTRAINT webhook_deliveries_schema_version_check;
ALTER TABLE webhook_deliveries
    ADD CONSTRAINT webhook_deliveries_schema_version_check CHECK (schema_version IN (1, 2));

COMMENT ON COLUMN webhooks.events IS 'Notification event types of the contract: post.approved, post.rejected, reply.removed, reply.flagged, blog_post_rejected (version 1); room.member_added, room.member_removed (version 2)';

-- Reverse the room_entries cutover: restore messages / room_events as real tables and
-- copy back everything written to the timeline after the cutover, so no entry is lost.
DROP TRIGGER IF EXISTS room_events_view_write ON room_events;
DROP TRIGGER IF EXISTS messages_view_write ON messages;
DROP VIEW IF EXISTS room_events;
DROP VIEW IF EXISTS messages;
DROP FUNCTION IF EXISTS room_events_view_write();
DROP FUNCTION IF EXISTS messages_view_write();

ALTER TABLE rooms DROP CONSTRAINT IF EXISTS rooms_result_message_id_fkey;
ALTER TABLE legacy_messages RENAME TO messages;
ALTER TABLE legacy_room_events RENAME TO room_events;
ALTER TABLE messages ALTER COLUMN id SET DEFAULT nextval('messages_id_seq');

-- Post-cutover messages (and edits to archived ones) come back with their ids.
UPDATE messages m SET
    content = e.body, content_type = e.content_type, metadata = e.extension,
    reply_to_entry_id = e.reply_to_entry_id, addressed_member_ids = e.addressed_member_ids,
    supersedes_entry_id = e.supersedes_entry_id, pinned_at = e.pinned_at, deleted_at = e.deleted_at
FROM room_entries e
WHERE e.id = m.id AND e.kind = 'message';

INSERT INTO messages
    (id, room_id, author_type, author_id, agent_name, content, content_type, metadata,
     sequence_num, created_at, deleted_at, reply_to_entry_id, addressed_member_ids,
     client_entry_id, pinned_at, supersedes_entry_id)
SELECT e.id, e.room_id, COALESCE(e.author_type, 'agent'), e.author_id, LEFT(e.actor_label, 100),
       e.body, e.content_type, e.extension, e.sequence, e.created_at, e.deleted_at,
       e.reply_to_entry_id, e.addressed_member_ids, e.client_entry_id, e.pinned_at,
       e.supersedes_entry_id
FROM room_entries e
WHERE e.kind = 'message' AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.id = e.id)
ORDER BY e.id;

INSERT INTO room_events (room_id, event_type, issue, actor, payload, created_at)
SELECT e.room_id, e.event_type, e.issue, e.actor_label, e.extension, e.created_at
FROM room_entries e
WHERE e.kind = 'event'
  AND NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                  WHERE lm.legacy_kind = 'event' AND lm.entry_id = e.id)
ORDER BY e.sequence;

SELECT setval('messages_id_seq', GREATEST((SELECT COALESCE(MAX(id), 0) FROM messages), 1));

ALTER TABLE rooms ADD CONSTRAINT rooms_result_message_id_fkey
    FOREIGN KEY (result_message_id) REFERENCES messages(id) ON DELETE SET NULL;

DROP TRIGGER IF EXISTS room_entries_allocate_before_insert ON room_entries;
DROP FUNCTION IF EXISTS room_entries_allocate();
DROP FUNCTION IF EXISTS room_entries_backfill_legacy();
DROP INDEX IF EXISTS idx_room_entries_room_created;
DROP INDEX IF EXISTS idx_room_entries_message_id;
DROP INDEX IF EXISTS idx_room_entries_event_recent;
DROP INDEX IF EXISTS idx_room_entries_room_pinned;
DROP INDEX IF EXISTS idx_room_entries_supersedes;

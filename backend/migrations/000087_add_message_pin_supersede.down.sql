DROP INDEX IF EXISTS idx_messages_supersedes;
DROP INDEX IF EXISTS idx_messages_room_pinned;
ALTER TABLE messages DROP COLUMN IF EXISTS supersedes_entry_id;
ALTER TABLE messages DROP COLUMN IF EXISTS pinned_at;

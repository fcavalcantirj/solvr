-- Add message pinning and superseding to the room timeline (task: support the
-- planner/executor review loop without imposing a heavyweight task system).
--
-- pinned_at: when set, this message is a pinned directive or result that
--   participants surface without scanning the whole transcript. NULL = not pinned.
--   Messages are immutable, so a pin references an exact version of the content.
--
-- supersedes_entry_id: when set, this message is a revised directive/result that
--   replaces an earlier message in the same room. The earlier message stays in
--   history; the reference makes the supersession explicit rather than deleting
--   the original. ON DELETE SET NULL so soft-deletion of the original never
--   orphans a dangling FK (soft delete does not trigger it; a hard delete clears it).
ALTER TABLE messages ADD COLUMN pinned_at TIMESTAMPTZ;
ALTER TABLE messages ADD COLUMN supersedes_entry_id BIGINT REFERENCES messages(id) ON DELETE SET NULL;

-- Fetch a room's pinned directives/results newest-first without scanning history.
CREATE INDEX idx_messages_room_pinned ON messages (room_id, id DESC)
    WHERE pinned_at IS NOT NULL AND deleted_at IS NULL;

-- Find the message that superseded a given one (the forward pointer).
CREATE INDEX idx_messages_supersedes ON messages (supersedes_entry_id)
    WHERE supersedes_entry_id IS NOT NULL AND deleted_at IS NULL;

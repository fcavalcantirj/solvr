-- Separate finished rooms from live presence (task: "Separate finished rooms
-- from live presence and retain useful public transcripts").
--
-- A room owner can mark a collaboration Finished. This records an ARCHIVED state
-- (archived_at) with an optional pointer to the room message that captured the
-- result (result_message_id). Archiving is DISTINCT from expiry and deletion:
--   * archived_at  -> the collaboration is finished but the transcript stays
--                     readable at its original URL, labeled Finished not Live;
--                     new messages/joins are refused until an owner reopens it.
--   * expires_at   -> retention window; DeleteExpiredRooms still hard-deletes past it.
--   * deleted_at   -> soft delete; the room disappears from public indexes.
-- Reopening simply clears archived_at.
--
-- result_message_id references messages(id); ON DELETE SET NULL so pruning a
-- message never dangles the pointer.
ALTER TABLE rooms ADD COLUMN archived_at TIMESTAMPTZ;
ALTER TABLE rooms ADD COLUMN result_message_id BIGINT REFERENCES messages(id) ON DELETE SET NULL;

CREATE INDEX idx_rooms_archived ON rooms (archived_at)
    WHERE archived_at IS NOT NULL;

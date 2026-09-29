-- Room activity is a projection of the room timeline (idx 77: counters and projections are
-- rebuildable from authoritative records). room_entries is the record; the rooms columns
-- are derived from it:
--
--   rooms.message_count  = the room's live (deleted_at IS NULL) message entries
--   rooms.last_active_at = the later of the room's created_at and its newest message entry;
--                          a soft-deleted message still happened, so it still counts
--
-- The API used to bump both in separate statements AFTER the entry had committed
-- (IncrementMessageCount, UpdateActivity), so a request whose context or connection ended
-- between the two left the room behind for good, with nothing that could recompute it.
-- These triggers move them inside the entry write's own transaction instead: a committed
-- entry is counted, a rolled-back one is not, and an idempotent replay (no new row) counts
-- nothing. Only the message kind counts; events are neither messages nor activity.
-- updated_at (the room ETag) advances with every change to the projection, as it did
-- when the API bumped it.
--
-- REBUILD PATH (operator, READ COMMITTED):
--   SELECT * FROM room_activity_drift();          -- rooms whose stored values differ
--   SELECT rebuild_room_activity();               -- repair every room; returns the count
--   SELECT rebuild_room_activity('<room uuid>');  -- repair one room
-- The rebuild holds the rooms' row locks while it recounts. A message insert takes the
-- same lock before it writes (room_entries_allocate), so no write can land between the
-- recount and the repair. A consistent room is not rewritten; a second rebuild returns 0.

CREATE OR REPLACE FUNCTION room_entries_activity() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE rooms
           SET message_count  = message_count + CASE WHEN NEW.deleted_at IS NULL THEN 1 ELSE 0 END,
               last_active_at = GREATEST(last_active_at, NEW.created_at),
               updated_at     = GREATEST(updated_at, clock_timestamp())
         WHERE id = NEW.room_id;
        RETURN NULL;
    END IF;

    IF TG_OP = 'DELETE' THEN
        -- The record is gone, so the newest remaining message is the latest activity.
        -- When the room itself is being deleted (cascade) this matches no row.
        UPDATE rooms r
           SET message_count  = GREATEST(r.message_count - CASE WHEN OLD.deleted_at IS NULL THEN 1 ELSE 0 END, 0),
               last_active_at = GREATEST(r.created_at, COALESCE(
                                    (SELECT MAX(e.created_at) FROM room_entries e
                                      WHERE e.room_id = r.id AND e.kind = 'message'),
                                    r.created_at)),
               updated_at     = GREATEST(r.updated_at, clock_timestamp())
         WHERE r.id = OLD.room_id;
        RETURN NULL;
    END IF;

    -- UPDATE: a message soft-deleted or restored (the trigger's WHEN clause).
    UPDATE rooms
       SET message_count = GREATEST(message_count + CASE WHEN NEW.deleted_at IS NULL THEN 1 ELSE -1 END, 0),
           updated_at    = GREATEST(updated_at, clock_timestamp())
     WHERE id = NEW.room_id;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS room_entries_activity_insert ON room_entries;
CREATE TRIGGER room_entries_activity_insert
    AFTER INSERT ON room_entries
    FOR EACH ROW
    WHEN (NEW.kind = 'message')
    EXECUTE FUNCTION room_entries_activity();

DROP TRIGGER IF EXISTS room_entries_activity_soft_delete ON room_entries;
CREATE TRIGGER room_entries_activity_soft_delete
    AFTER UPDATE OF deleted_at ON room_entries
    FOR EACH ROW
    WHEN (NEW.kind = 'message' AND (OLD.deleted_at IS NULL) <> (NEW.deleted_at IS NULL))
    EXECUTE FUNCTION room_entries_activity();

DROP TRIGGER IF EXISTS room_entries_activity_delete ON room_entries;
CREATE TRIGGER room_entries_activity_delete
    AFTER DELETE ON room_entries
    FOR EACH ROW
    WHEN (OLD.kind = 'message')
    EXECUTE FUNCTION room_entries_activity();

-- Read-only reconciliation: the rooms (one room when p_room_id is given) whose stored
-- activity differs from their timeline, with both values.
CREATE OR REPLACE FUNCTION room_activity_drift(p_room_id uuid DEFAULT NULL)
RETURNS TABLE (room_id uuid,
               stored_message_count integer, timeline_message_count integer,
               stored_last_active_at timestamptz, timeline_last_active_at timestamptz)
LANGUAGE sql STABLE AS $$
    SELECT r.id, r.message_count, t.live_messages, r.last_active_at, t.last_active_at
    FROM rooms r
    CROSS JOIN LATERAL (
        SELECT COUNT(*) FILTER (WHERE e.deleted_at IS NULL)::integer AS live_messages,
               GREATEST(r.created_at, COALESCE(MAX(e.created_at), r.created_at)) AS last_active_at
        FROM room_entries e
        WHERE e.room_id = r.id AND e.kind = 'message'
    ) t
    WHERE (p_room_id IS NULL OR r.id = p_room_id)
      AND (r.message_count <> t.live_messages OR r.last_active_at <> t.last_active_at)
    ORDER BY r.id;
$$;

CREATE OR REPLACE FUNCTION rebuild_room_activity(p_room_id uuid DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    repaired integer;
BEGIN
    -- Lock first, recount after: the UPDATE below runs on a snapshot taken once every
    -- in-flight message insert of these rooms has committed, and new ones wait for us.
    PERFORM 1 FROM rooms
     WHERE p_room_id IS NULL OR id = p_room_id
     ORDER BY id
       FOR NO KEY UPDATE;

    UPDATE rooms r
       SET message_count  = d.timeline_message_count,
           last_active_at = d.timeline_last_active_at,
           updated_at     = GREATEST(r.updated_at, clock_timestamp())
      FROM room_activity_drift(p_room_id) d
     WHERE r.id = d.room_id;
    GET DIAGNOSTICS repaired = ROW_COUNT;
    RETURN repaired;
END;
$$;

COMMENT ON FUNCTION room_activity_drift(uuid) IS
    'Rooms whose stored message_count/last_active_at differ from room_entries (000112). Read-only.';
COMMENT ON FUNCTION rebuild_room_activity(uuid) IS
    'Recompute rooms.message_count/last_active_at from room_entries (all rooms when NULL); returns rooms repaired (000112).';

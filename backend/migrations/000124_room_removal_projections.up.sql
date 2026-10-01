-- Public room projections follow the rooms and agents they show (idx 77 step 5: visibility and
-- deletion changes remove public projections promptly). Measured before this migration (idx 77
-- slice 11 spike, live API):
--
--   * GET /v1/overview names public rooms (activity feed, previews, recent rooms: slug, display
--     name, description) from a 30 s snapshot on each instance. Only RoomHandler announced room
--     changes, and only visibility, archive, reopen and delete: a room made private,
--     soft-deleted, row-deleted or renamed by SQL, and a rename through PATCH, stayed on the
--     overview until the snapshot ran out while GET /v1/rooms/{slug} already answered 403/404.
--   * A self-deleted or banned agent can no longer join or heartbeat (its room tokens stop
--     resolving, idx 75), but its presence rows stayed: GET /v1/rooms/{slug} and
--     /v1/rooms/{slug}/agents listed it as online, card included, until its TTL ran out (600 s
--     by default), and the homepage counted it.
--
-- 1. Rooms announce, at commit, every write that changes how a PUBLIC room is shown (before or
--    after the write), whoever the writer is (handlers, the presence reaper's expiry delete, SQL),
--    with 000123's overview_changed_notify(): the empty notice on solvr_overview_changed that
--    every instance's room listener turns into a snapshot drop. Postgres folds identical notices
--    of one transaction, so a bulk statement sends one. Quiet on purpose: messages
--    (message_count, last_active_at), tags, category, a column rewritten to its own value, and
--    private or already deleted rooms (never named).
--
-- 2. An agent's removal (deleted_at set: DELETE /v1/agents/me, POST /admin/bans, SQL) ends its
--    presence in the same transaction and announces each leave on solvr_room_presence as a
--    hub.PresenceChange with origin 'database', so every instance drops it from its in-memory
--    registry and shows presence_leave on its streams, as for a leave made through another
--    instance. Its memberships and room tokens are left as they are (the tokens already stop
--    resolving). A dry-run ban rolls back: nothing ends, nothing is sent.
--
-- Rebuild path: none needed. Both are wakeups or removals of ephemeral state; the overview is
-- re-read from the tables on the next request and presence is re-created by the next join.
DROP TRIGGER IF EXISTS rooms_overview_changed_update ON rooms;
CREATE TRIGGER rooms_overview_changed_update
    AFTER UPDATE OF is_private, deleted_at, archived_at, expires_at, slug, display_name, description ON rooms
    FOR EACH ROW
    WHEN (((NOT OLD.is_private AND OLD.deleted_at IS NULL) OR (NOT NEW.is_private AND NEW.deleted_at IS NULL))
          AND (OLD.is_private IS DISTINCT FROM NEW.is_private
               OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
               OR OLD.archived_at IS DISTINCT FROM NEW.archived_at
               OR OLD.expires_at IS DISTINCT FROM NEW.expires_at
               OR OLD.slug IS DISTINCT FROM NEW.slug
               OR OLD.display_name IS DISTINCT FROM NEW.display_name
               OR OLD.description IS DISTINCT FROM NEW.description))
    EXECUTE FUNCTION overview_changed_notify();

DROP TRIGGER IF EXISTS rooms_overview_changed_delete ON rooms;
CREATE TRIGGER rooms_overview_changed_delete
    AFTER DELETE ON rooms
    FOR EACH ROW
    WHEN (NOT OLD.is_private AND OLD.deleted_at IS NULL)
    EXECUTE FUNCTION overview_changed_notify();

CREATE OR REPLACE FUNCTION agent_removed_presence_end() RETURNS trigger AS $$
DECLARE
    gone RECORD;
BEGIN
    FOR gone IN DELETE FROM agent_presence WHERE agent_id = NEW.id RETURNING room_id, agent_name LOOP
        PERFORM pg_notify('solvr_room_presence', json_build_object(
            'room_id', gone.room_id, 'agent_name', gone.agent_name, 'joined', FALSE, 'origin', 'database')::text);
    END LOOP;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS agents_removed_presence_end ON agents;
CREATE TRIGGER agents_removed_presence_end
    AFTER UPDATE OF deleted_at ON agents
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION agent_removed_presence_end();

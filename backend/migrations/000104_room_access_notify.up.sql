-- Room access changes reach streams that are already open, on every API instance.
--
-- An established room stream was authorized once, when it opened. These triggers announce
-- (at commit, payload = room id) every change that can take read access away from a
-- caller: a membership revoked or re-roled, a per-agent room token deleted or replaced, a
-- room made private, soft-deleted or given a new expiry, or a room row removed. Each
-- listening instance re-authorizes its open streams of that room and ends the ones whose
-- caller lost access. Like solvr_room_entries the notice is only a wakeup: the decision is
-- re-read from the database, and a notice lost while a listener reconnects is covered by
-- re-checking every open stream on (re)LISTEN and on each stream heartbeat.
CREATE OR REPLACE FUNCTION room_access_notify() RETURNS trigger AS $$
DECLARE
    rid uuid;
BEGIN
    IF TG_TABLE_NAME = 'rooms' THEN
        rid := CASE WHEN TG_OP = 'DELETE' THEN OLD.id ELSE NEW.id END;
    ELSE
        rid := CASE WHEN TG_OP = 'DELETE' THEN OLD.room_id ELSE NEW.room_id END;
    END IF;
    PERFORM pg_notify('solvr_room_access', rid::text);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS room_members_access_notify_update ON room_members;
CREATE TRIGGER room_members_access_notify_update
    AFTER UPDATE OF revoked_at, role ON room_members
    FOR EACH ROW
    WHEN (OLD.revoked_at IS DISTINCT FROM NEW.revoked_at OR OLD.role IS DISTINCT FROM NEW.role)
    EXECUTE FUNCTION room_access_notify();

DROP TRIGGER IF EXISTS room_members_access_notify_delete ON room_members;
CREATE TRIGGER room_members_access_notify_delete
    AFTER DELETE ON room_members
    FOR EACH ROW EXECUTE FUNCTION room_access_notify();

DROP TRIGGER IF EXISTS room_agent_tokens_access_notify_update ON room_agent_tokens;
CREATE TRIGGER room_agent_tokens_access_notify_update
    AFTER UPDATE OF token_hash, expires_at ON room_agent_tokens
    FOR EACH ROW
    WHEN (OLD.token_hash IS DISTINCT FROM NEW.token_hash OR OLD.expires_at IS DISTINCT FROM NEW.expires_at)
    EXECUTE FUNCTION room_access_notify();

DROP TRIGGER IF EXISTS room_agent_tokens_access_notify_delete ON room_agent_tokens;
CREATE TRIGGER room_agent_tokens_access_notify_delete
    AFTER DELETE ON room_agent_tokens
    FOR EACH ROW EXECUTE FUNCTION room_access_notify();

DROP TRIGGER IF EXISTS rooms_access_notify_update ON rooms;
CREATE TRIGGER rooms_access_notify_update
    AFTER UPDATE OF is_private, deleted_at, expires_at ON rooms
    FOR EACH ROW
    WHEN (OLD.is_private IS DISTINCT FROM NEW.is_private
          OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at
          OR OLD.expires_at IS DISTINCT FROM NEW.expires_at)
    EXECUTE FUNCTION room_access_notify();

DROP TRIGGER IF EXISTS rooms_access_notify_delete ON rooms;
CREATE TRIGGER rooms_access_notify_delete
    AFTER DELETE ON rooms
    FOR EACH ROW EXECUTE FUNCTION room_access_notify();

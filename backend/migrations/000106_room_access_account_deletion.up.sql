-- A deleted account reaches the room streams it left open (idx 75 step 3).
--
-- An established room stream is re-authorized against the account behind it, so a soft-deleted
-- human or agent stops reading (and its per-agent room tokens stop resolving) once the room
-- guards re-check. Those re-checks run on every access-change signal for the room and on every
-- 30 s heartbeat; this announces the deletion on the same channel as 000104, so an open stream
-- of the rooms the account was in ends in about a second, on every API instance, instead of at
-- the next heartbeat.
--
--   * an agent: every room where it holds an active membership (a per-agent room token row
--     implies one);
--   * a human: every room where it holds an active membership. A room it solely owned keeps
--     that owner row active on purpose (000100: an admin recovery restores ownership), so no
--     membership change announces it; this does.
--
-- Family access (an agent admitted through its linked human's ownership) has no row of its
-- own to announce and is covered by the heartbeat re-check. Like every notice on this
-- channel this is only a wakeup: the decision is re-read from the database.
CREATE OR REPLACE FUNCTION account_deleted_access_notify() RETURNS trigger AS $$
BEGIN
    IF TG_TABLE_NAME = 'users' THEN
        PERFORM pg_notify('solvr_room_access', m.room_id::text)
        FROM (SELECT DISTINCT room_id FROM room_members
              WHERE user_id = NEW.id AND revoked_at IS NULL) m;
    ELSE
        PERFORM pg_notify('solvr_room_access', m.room_id::text)
        FROM (SELECT DISTINCT room_id FROM room_members
              WHERE agent_id = NEW.id AND revoked_at IS NULL) m;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS users_deleted_access_notify ON users;
CREATE TRIGGER users_deleted_access_notify
    AFTER UPDATE OF deleted_at ON users
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION account_deleted_access_notify();

DROP TRIGGER IF EXISTS agents_deleted_access_notify ON agents;
CREATE TRIGGER agents_deleted_access_notify
    AFTER UPDATE OF deleted_at ON agents
    FOR EACH ROW
    WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION account_deleted_access_notify();

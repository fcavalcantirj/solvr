-- room_members becomes the single room-membership authority for humans AND agents.
--
-- Before: agents only (PK room_id+agent_id); a human's ownership lived solely in
-- rooms.owner_id, and removing a member erased the row.
-- After:
--   * exactly one actor per row: agent_id (agents.id) XOR user_id (users.id);
--   * role owner|member plus an optional free-text display_role ("planner", ...);
--   * created_at is the join timestamp, revoked_at the revocation timestamp. A removed
--     member keeps its row with revoked_at set; an explicit readmission clears it.
--     UNIQUE (room_id, agent_id) and UNIQUE (room_id, user_id) give one membership row,
--     hence at most one active membership, per actor and room;
--   * every rooms.owner_id is backfilled (and kept in sync by trigger during the
--     compatibility window) as an active human 'owner' membership, created in the same
--     transaction as the room;
--   * a live room can never lose its last active owner through a direct revoke/demote/
--     delete (ERRCODE check_violation, CONSTRAINT room_members_final_owner). Deleting
--     the room releases it. When the owner's ACCOUNT is hard-deleted (agent or human),
--     the now-ownerless room is ARCHIVED instead: its transcript stays readable and new
--     activity is refused, which is the documented terminal state;
--   * individual room credentials are bound to the membership: revoking a member
--     deletes its per-agent room token (room_agent_tokens).
-- Agent unlinking (agents.human_id -> NULL) does not touch memberships: the agent keeps
-- its own memberships and the human keeps only the memberships it holds itself.

ALTER TABLE room_members ADD COLUMN id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE room_members DROP CONSTRAINT room_members_pkey;
ALTER TABLE room_members ADD CONSTRAINT room_members_pkey PRIMARY KEY (id);
ALTER TABLE room_members ALTER COLUMN agent_id DROP NOT NULL;
ALTER TABLE room_members ADD CONSTRAINT room_members_room_agent_key UNIQUE (room_id, agent_id);

ALTER TABLE room_members
    ADD COLUMN user_id      UUID REFERENCES users(id) ON DELETE CASCADE,
    ADD COLUMN display_role TEXT CHECK (display_role IS NULL OR char_length(display_role) BETWEEN 1 AND 64),
    ADD COLUMN revoked_at   TIMESTAMPTZ;

ALTER TABLE room_members ADD CONSTRAINT room_members_room_user_key UNIQUE (room_id, user_id);
ALTER TABLE room_members ADD CONSTRAINT room_members_exactly_one_actor
    CHECK (num_nonnulls(agent_id, user_id) = 1);

CREATE INDEX idx_room_members_user ON room_members (user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_room_members_active_owners ON room_members (room_id)
    WHERE role = 'owner' AND revoked_at IS NULL;

-- Backfill human ownership from rooms.owner_id (agent owners already have rows).
INSERT INTO room_members (room_id, user_id, role, added_by, created_at)
SELECT r.id, r.owner_id, 'owner', 'system', r.created_at
FROM rooms r
WHERE r.owner_id IS NOT NULL
ON CONFLICT (room_id, user_id) DO NOTHING;

-- Compatibility window: writers still set rooms.owner_id (room creation, claim backfill).
-- Mirror every non-null owner_id into an active human owner membership atomically.
CREATE FUNCTION rooms_sync_human_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    INSERT INTO room_members (room_id, user_id, role, added_by)
    VALUES (NEW.id, NEW.owner_id, 'owner', 'system')
    ON CONFLICT (room_id, user_id) DO UPDATE SET role = 'owner', revoked_at = NULL;
    RETURN NULL;
END;
$$;

CREATE TRIGGER rooms_sync_human_owner
    AFTER INSERT OR UPDATE OF owner_id ON rooms
    FOR EACH ROW WHEN (NEW.owner_id IS NOT NULL)
    EXECUTE FUNCTION rooms_sync_human_owner();

-- Final-owner guard. Deferred so an ownership transfer may promote and demote in either
-- order inside one transaction; evaluated against the committed-to-be state.
CREATE FUNCTION room_members_keep_owner() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    room_deleted TIMESTAMPTZ;
BEGIN
    IF OLD.role <> 'owner' OR OLD.revoked_at IS NOT NULL THEN
        RETURN NULL;
    END IF;
    SELECT deleted_at INTO room_deleted FROM rooms WHERE id = OLD.room_id;
    IF NOT FOUND OR room_deleted IS NOT NULL THEN
        RETURN NULL; -- the room itself is being (or has been) deleted
    END IF;
    IF EXISTS (SELECT 1 FROM room_members
               WHERE room_id = OLD.room_id AND role = 'owner' AND revoked_at IS NULL) THEN
        RETURN NULL;
    END IF;
    IF TG_OP = 'DELETE' AND (
           (OLD.agent_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM agents WHERE id = OLD.agent_id))
        OR (OLD.user_id  IS NOT NULL AND NOT EXISTS (SELECT 1 FROM users  WHERE id = OLD.user_id))
    ) THEN
        -- The owner's account was deleted: archive rather than leave it unmanageable.
        UPDATE rooms SET archived_at = COALESCE(archived_at, NOW()), updated_at = NOW()
        WHERE id = OLD.room_id;
        RETURN NULL;
    END IF;
    RAISE EXCEPTION 'room % must keep at least one owner; transfer ownership or delete the room', OLD.room_id
        USING ERRCODE = 'check_violation', CONSTRAINT = 'room_members_final_owner';
END;
$$;

CREATE CONSTRAINT TRIGGER room_members_keep_owner
    AFTER UPDATE OR DELETE ON room_members
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION room_members_keep_owner();

-- Individual room credentials live only as long as the membership they were issued for.
CREATE FUNCTION room_members_revoke_credentials() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM room_agent_tokens WHERE room_id = NEW.room_id AND agent_id = NEW.agent_id;
    RETURN NULL;
END;
$$;

CREATE TRIGGER room_members_revoke_credentials
    AFTER UPDATE OF revoked_at ON room_members
    FOR EACH ROW
    WHEN (OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL AND NEW.agent_id IS NOT NULL)
    EXECUTE FUNCTION room_members_revoke_credentials();

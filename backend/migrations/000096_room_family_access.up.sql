-- Family access reads the membership authority (000095), not rooms.owner_id.
--
-- An agent has family access to a room while its CURRENT linked human (agents.human_id)
-- is a live account (users.deleted_at IS NULL) holding an ACTIVE owner membership there.
-- It is derived at request time and never stored, so no historical link can broaden it.
--
-- When family access is used to participate directly (handshake), the agent's membership
-- is materialized with access_source = 'family'. That row lives only as long as the
-- justification: it is revoked (and, via 000095, its per-agent room token deleted) when
--   * the agent's link to that human changes,
--   * the human stops being an active owner of the room (revoke, demote, row removal),
--   * the human's account is deleted (soft delete sets users.deleted_at).
-- An explicit owner grant (members API) marks the row 'direct'; a direct grant is never
-- downgraded by family materialization and survives all of the above.

ALTER TABLE room_members ADD COLUMN access_source TEXT NOT NULL DEFAULT 'direct'
    CHECK (access_source IN ('direct', 'family'));

CREATE INDEX idx_room_members_family ON room_members (agent_id)
    WHERE access_source = 'family' AND revoked_at IS NULL;

-- Link change on the agent: end every family grant it held.
CREATE FUNCTION agents_end_family_memberships() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE room_members SET revoked_at = NOW()
    WHERE agent_id = NEW.id AND access_source = 'family' AND revoked_at IS NULL;
    RETURN NULL;
END;
$$;

CREATE TRIGGER agents_end_family_memberships
    AFTER UPDATE OF human_id ON agents
    FOR EACH ROW WHEN (OLD.human_id IS DISTINCT FROM NEW.human_id)
    EXECUTE FUNCTION agents_end_family_memberships();

-- The human stops being an active owner of a room: end family grants in that room.
CREATE FUNCTION room_members_end_family_on_owner_loss() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.user_id IS NULL OR OLD.role <> 'owner' OR OLD.revoked_at IS NOT NULL THEN
        RETURN NULL;
    END IF;
    IF TG_OP = 'UPDATE' AND NEW.role = 'owner' AND NEW.revoked_at IS NULL THEN
        RETURN NULL;
    END IF;
    UPDATE room_members m SET revoked_at = NOW()
    FROM agents a
    WHERE m.room_id = OLD.room_id AND m.agent_id = a.id AND a.human_id = OLD.user_id
      AND m.access_source = 'family' AND m.revoked_at IS NULL;
    RETURN NULL;
END;
$$;

CREATE TRIGGER room_members_end_family_on_owner_loss
    AFTER UPDATE OF role, revoked_at OR DELETE ON room_members
    FOR EACH ROW EXECUTE FUNCTION room_members_end_family_on_owner_loss();

-- The human's account is deleted: end family grants of every agent linked to it.
CREATE FUNCTION users_end_family_memberships() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE room_members m SET revoked_at = NOW()
    FROM agents a
    WHERE m.agent_id = a.id AND a.human_id = NEW.id
      AND m.access_source = 'family' AND m.revoked_at IS NULL;
    RETURN NULL;
END;
$$;

CREATE TRIGGER users_end_family_memberships
    AFTER UPDATE OF deleted_at ON users
    FOR EACH ROW WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)
    EXECUTE FUNCTION users_end_family_memberships();

-- Bind room presence and per-agent room credentials to the membership authority.
--
-- Before: agent_presence was keyed by a free-text agent_name that any bearer could claim
-- (one agent could heartbeat, overwrite or remove another's presence), and a
-- room_agent_tokens row only referenced rooms + agents, not the membership that justified
-- it. After: both reference room_members(room_id, agent_id), a row can only be written
-- while that membership is ACTIVE, and revoking the membership removes both.

-- 1. Credentials: drop any token whose membership is gone or revoked (it could never have
--    been issued under the current handshake), then reference the membership.
DELETE FROM room_agent_tokens t
 WHERE NOT EXISTS (SELECT 1 FROM room_members m
                    WHERE m.room_id = t.room_id AND m.agent_id = t.agent_id
                      AND m.revoked_at IS NULL);

ALTER TABLE room_agent_tokens
    ADD CONSTRAINT room_agent_tokens_membership_fkey
    FOREIGN KEY (room_id, agent_id) REFERENCES room_members (room_id, agent_id) ON DELETE CASCADE;

-- 2. Presence: record WHICH member is present. Existing rows are bound when their
--    agent_name is exactly one active agent member's id or display name; the rest are
--    ephemeral (TTL minutes) leftovers of the free-text model and are dropped — the agent
--    reappears on its next /r/{slug}/join.
ALTER TABLE agent_presence ADD COLUMN agent_id VARCHAR(50);

UPDATE agent_presence p
   SET agent_id = (SELECT m.agent_id FROM room_members m JOIN agents a ON a.id = m.agent_id
                    WHERE m.room_id = p.room_id AND m.revoked_at IS NULL
                      AND (a.id = p.agent_name OR a.display_name = p.agent_name))
 WHERE (SELECT COUNT(*) FROM room_members m JOIN agents a ON a.id = m.agent_id
         WHERE m.room_id = p.room_id AND m.revoked_at IS NULL
           AND (a.id = p.agent_name OR a.display_name = p.agent_name)) = 1;

DELETE FROM agent_presence WHERE agent_id IS NULL;

-- Two legacy names that resolved to the same member: keep the most recently seen.
DELETE FROM agent_presence p
 USING agent_presence q
 WHERE p.room_id = q.room_id AND p.agent_id = q.agent_id
   AND (p.last_seen, p.id) < (q.last_seen, q.id);

ALTER TABLE agent_presence ALTER COLUMN agent_id SET NOT NULL;
ALTER TABLE agent_presence
    ADD CONSTRAINT agent_presence_room_agent_key UNIQUE (room_id, agent_id);
ALTER TABLE agent_presence
    ADD CONSTRAINT agent_presence_membership_fkey
    FOREIGN KEY (room_id, agent_id) REFERENCES room_members (room_id, agent_id) ON DELETE CASCADE;

-- 3. Writes require an ACTIVE membership (the FK alone accepts a revoked row).
CREATE FUNCTION room_member_active_required() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM room_members m
                    WHERE m.room_id = NEW.room_id AND m.agent_id = NEW.agent_id
                      AND m.revoked_at IS NULL) THEN
        RAISE EXCEPTION 'agent % is not an active member of room %', NEW.agent_id, NEW.room_id
            USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'room_member_active_required';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER agent_presence_member_active
    BEFORE INSERT OR UPDATE OF room_id, agent_id ON agent_presence
    FOR EACH ROW EXECUTE FUNCTION room_member_active_required();

CREATE TRIGGER room_agent_tokens_member_active
    BEFORE INSERT OR UPDATE OF room_id, agent_id ON room_agent_tokens
    FOR EACH ROW EXECUTE FUNCTION room_member_active_required();

-- 4. Revoking a membership ends the member's credential (000095) AND its presence.
CREATE OR REPLACE FUNCTION room_members_revoke_credentials() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM room_agent_tokens WHERE room_id = NEW.room_id AND agent_id = NEW.agent_id;
    DELETE FROM agent_presence WHERE room_id = NEW.room_id AND agent_id = NEW.agent_id;
    RETURN NULL;
END;
$$;

-- Restore the 000098 shape: presence keyed by agent_name only, credentials referencing
-- rooms + agents only. Bound rows are kept (agent_id is simply dropped).
CREATE OR REPLACE FUNCTION room_members_revoke_credentials() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    DELETE FROM room_agent_tokens WHERE room_id = NEW.room_id AND agent_id = NEW.agent_id;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS room_agent_tokens_member_active ON room_agent_tokens;
DROP TRIGGER IF EXISTS agent_presence_member_active ON agent_presence;
DROP FUNCTION IF EXISTS room_member_active_required();

ALTER TABLE agent_presence DROP CONSTRAINT IF EXISTS agent_presence_membership_fkey;
ALTER TABLE agent_presence DROP CONSTRAINT IF EXISTS agent_presence_room_agent_key;
ALTER TABLE agent_presence DROP COLUMN IF EXISTS agent_id;

ALTER TABLE room_agent_tokens DROP CONSTRAINT IF EXISTS room_agent_tokens_membership_fkey;

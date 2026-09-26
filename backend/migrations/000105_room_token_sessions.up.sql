-- One agent, several sessions, explicit rotation (idx 75 step 1).
--
-- Until now room_agent_tokens held ONE token per (room, agent): any handshake replaced the
-- previous token, so a second session of the same agent that merely followed the connect
-- instructions ("handshake for your own token") silently killed the first session's
-- credential, and recovering re-killed the other one.
--
-- After: every handshake ADDS a token (a session); the agent's earlier tokens keep working.
-- Only an explicit rotation replaces them, and it marks them instead of deleting them
-- (rotated_at) so a replaced token's holder is told CREDENTIAL_ROTATED and can recover by
-- handshaking again, rather than getting the same 401 as a token that never existed.
-- Revoking a member (000095 trigger) or the owner revoking the agent's token still DELETES
-- every row for the pair, rotated ones included: that is not a rotation and stays a plain 401.

ALTER TABLE room_agent_tokens ADD COLUMN rotated_at TIMESTAMPTZ;

-- (room_id, agent_id) is no longer unique. The token hash already is (idx_room_agent_tokens_hash),
-- so it becomes the primary key.
ALTER TABLE room_agent_tokens DROP CONSTRAINT room_agent_tokens_pkey;
ALTER TABLE room_agent_tokens
    ADD CONSTRAINT room_agent_tokens_pkey PRIMARY KEY USING INDEX idx_room_agent_tokens_hash;

CREATE INDEX idx_room_agent_tokens_room_agent ON room_agent_tokens (room_id, agent_id);

-- A rotation is an UPDATE of rotated_at: it must reach the open streams of the replaced
-- tokens on every API instance, like a deleted token does (000104).
DROP TRIGGER IF EXISTS room_agent_tokens_access_notify_update ON room_agent_tokens;
CREATE TRIGGER room_agent_tokens_access_notify_update
    AFTER UPDATE OF token_hash, expires_at, rotated_at ON room_agent_tokens
    FOR EACH ROW
    WHEN (OLD.token_hash IS DISTINCT FROM NEW.token_hash
          OR OLD.expires_at IS DISTINCT FROM NEW.expires_at
          OR OLD.rotated_at IS DISTINCT FROM NEW.rotated_at)
    EXECUTE FUNCTION room_access_notify();

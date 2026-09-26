-- Back to one token per (room, agent): forget replaced tokens, keep the newest live token of
-- each pair, restore the (room_id, agent_id) primary key and the 000104 notify trigger.
DELETE FROM room_agent_tokens WHERE rotated_at IS NOT NULL;

DELETE FROM room_agent_tokens t
 USING room_agent_tokens n
 WHERE t.room_id = n.room_id AND t.agent_id = n.agent_id
   AND (t.created_at, t.token_hash) < (n.created_at, n.token_hash);

DROP TRIGGER IF EXISTS room_agent_tokens_access_notify_update ON room_agent_tokens;
CREATE TRIGGER room_agent_tokens_access_notify_update
    AFTER UPDATE OF token_hash, expires_at ON room_agent_tokens
    FOR EACH ROW
    WHEN (OLD.token_hash IS DISTINCT FROM NEW.token_hash OR OLD.expires_at IS DISTINCT FROM NEW.expires_at)
    EXECUTE FUNCTION room_access_notify();

DROP INDEX IF EXISTS idx_room_agent_tokens_room_agent;

ALTER TABLE room_agent_tokens DROP CONSTRAINT room_agent_tokens_pkey;
CREATE UNIQUE INDEX idx_room_agent_tokens_hash ON room_agent_tokens (token_hash);
ALTER TABLE room_agent_tokens ADD CONSTRAINT room_agent_tokens_pkey PRIMARY KEY (room_id, agent_id);

ALTER TABLE room_agent_tokens DROP COLUMN rotated_at;

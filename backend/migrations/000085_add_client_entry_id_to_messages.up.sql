-- Add client_entry_id for idempotent message writes (task: dependable message
-- delivery and recovery without a mandatory persistent process).
--
-- A retrying agent that lost the response to its write re-sends the same
-- client_entry_id. The unique index below makes a duplicate write for the same
-- authenticated author in the same room a no-op that returns the existing entry
-- instead of creating a second message.
--
-- The key is (room_id, author_id, client_entry_id): idempotency applies to the
-- same AUTHENTICATED author (author_id is the trustworthy per-agent identity
-- stamped from the room token) within one room. The spoofable agent_name is never
-- part of the key, so a shared-token post (author_id NULL) is never deduplicated.
ALTER TABLE messages ADD COLUMN client_entry_id VARCHAR(255);

CREATE UNIQUE INDEX idx_messages_client_entry
    ON messages (room_id, author_id, client_entry_id)
    WHERE client_entry_id IS NOT NULL AND author_id IS NOT NULL AND deleted_at IS NULL;

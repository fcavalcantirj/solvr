-- Add group reply and addressing fields to messages (task 16).
--
-- reply_to_entry_id: allows a message to reference another message in the same room.
--   Enables reply threading while keeping the message visible to all room participants.
--   NULL = message is not a reply to another message.
--
-- addressed_member_ids: JSON array of agent IDs this message is addressed to.
--   Used as a routing hint; all room participants still see the message.
--   NULL = message is not specifically addressed to any members (broadcast).

ALTER TABLE messages ADD COLUMN reply_to_entry_id BIGINT REFERENCES messages(id) ON DELETE SET NULL;
ALTER TABLE messages ADD COLUMN addressed_member_ids JSONB;

-- Constraint: reply_to_entry_id must reference a message in the same room.
-- This is enforced in the application layer during validation.

-- Index for finding messages that are replies to a specific message.
CREATE INDEX idx_messages_reply_to_entry_id ON messages (reply_to_entry_id) WHERE deleted_at IS NULL;

-- Index for finding messages addressed to a specific member (via JSONB contains).
CREATE INDEX idx_messages_addressed_member_ids ON messages USING GIN (addressed_member_ids);

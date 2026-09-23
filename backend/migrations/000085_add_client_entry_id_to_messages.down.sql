DROP INDEX IF EXISTS idx_messages_client_entry;
ALTER TABLE messages DROP COLUMN IF EXISTS client_entry_id;

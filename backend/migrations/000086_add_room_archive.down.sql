DROP INDEX IF EXISTS idx_rooms_archived;
ALTER TABLE rooms DROP COLUMN IF EXISTS result_message_id;
ALTER TABLE rooms DROP COLUMN IF EXISTS archived_at;

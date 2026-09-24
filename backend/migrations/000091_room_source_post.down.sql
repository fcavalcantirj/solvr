DROP INDEX IF EXISTS idx_rooms_source_post;
ALTER TABLE rooms DROP COLUMN IF EXISTS source_post_id;

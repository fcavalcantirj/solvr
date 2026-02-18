-- Remove crystallization fields from posts table.

DROP INDEX IF EXISTS idx_posts_crystallization_cid;
ALTER TABLE posts DROP COLUMN IF EXISTS crystallized_at;
ALTER TABLE posts DROP COLUMN IF EXISTS crystallization_cid;

DROP INDEX IF EXISTS idx_posts_idempotency_key;
ALTER TABLE posts DROP COLUMN IF EXISTS idempotency_key;

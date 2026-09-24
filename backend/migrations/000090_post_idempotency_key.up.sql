-- Save-as-post idempotency (task: "Turn an intentional room outcome into a reusable Post").
-- A retried "Save as post" carrying the same Idempotency-Key from the same author must
-- return the existing draft instead of creating a duplicate outcome draft.
ALTER TABLE posts ADD COLUMN IF NOT EXISTS idempotency_key TEXT;

-- Scope the key to the author + operation: one live post per (author, key).
CREATE UNIQUE INDEX IF NOT EXISTS idx_posts_idempotency_key
    ON posts (posted_by_type, posted_by_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND deleted_at IS NULL;

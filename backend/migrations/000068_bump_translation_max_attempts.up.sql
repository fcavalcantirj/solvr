-- Bump translation max attempts from 3 to 5 to recover stuck posts.
-- The partial index must match the query filter in posts_translation.go.
DROP INDEX IF EXISTS idx_posts_needs_translation;
CREATE INDEX idx_posts_needs_translation
  ON posts (created_at ASC)
  WHERE status = 'draft' AND original_language IS NOT NULL AND translation_attempts < 5;

-- Revert to original max attempts of 3.
DROP INDEX IF EXISTS idx_posts_needs_translation;
CREATE INDEX idx_posts_needs_translation
  ON posts (created_at ASC)
  WHERE status = 'draft' AND original_language IS NOT NULL AND translation_attempts < 3;

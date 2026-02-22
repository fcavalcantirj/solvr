DROP INDEX IF EXISTS idx_posts_needs_translation;

ALTER TABLE posts
  DROP COLUMN IF EXISTS original_language,
  DROP COLUMN IF EXISTS original_title,
  DROP COLUMN IF EXISTS original_description,
  DROP COLUMN IF EXISTS translation_attempts;

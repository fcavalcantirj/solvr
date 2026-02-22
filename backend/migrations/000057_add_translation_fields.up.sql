ALTER TABLE posts
  ADD COLUMN original_language    VARCHAR(50),
  ADD COLUMN original_title       VARCHAR(200),
  ADD COLUMN original_description TEXT,
  ADD COLUMN translation_attempts INT NOT NULL DEFAULT 0;

CREATE INDEX idx_posts_needs_translation
  ON posts (created_at ASC)
  WHERE status = 'draft' AND original_language IS NOT NULL AND translation_attempts < 3;

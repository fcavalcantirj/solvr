-- Add separate full-text search indexes for title and description columns
-- These enable potential future optimization for title-only or description-only searches
--
-- NOTE: idx_posts_content_tsvector (combined title + description) already exists
-- as idx_posts_search since migration 000003. Creating a duplicate would waste
-- ~100-200MB of disk space and add unnecessary write overhead.

-- Title-only full-text search index
CREATE INDEX IF NOT EXISTS idx_posts_title_tsvector
  ON posts USING GIN (to_tsvector('english', title));

-- Description-only full-text search index
CREATE INDEX IF NOT EXISTS idx_posts_description_tsvector
  ON posts USING GIN (to_tsvector('english', description));

-- The combined index (title || description) already exists as idx_posts_search
-- Created in migration 000003_create_posts.up.sql:
--   CREATE INDEX idx_posts_search ON posts USING GIN(to_tsvector('english', title || ' ' || description));

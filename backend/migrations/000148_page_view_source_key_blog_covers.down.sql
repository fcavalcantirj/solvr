ALTER TABLE blog_posts DROP COLUMN IF EXISTS cover_attempts;
DROP TABLE IF EXISTS blog_post_covers;
DROP INDEX IF EXISTS idx_page_view_events_source_minute;
ALTER TABLE page_view_events DROP COLUMN IF EXISTS source_key;

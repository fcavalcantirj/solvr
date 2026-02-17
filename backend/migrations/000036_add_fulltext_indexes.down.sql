-- Remove title and description full-text search indexes
DROP INDEX IF EXISTS idx_posts_title_tsvector;
DROP INDEX IF EXISTS idx_posts_description_tsvector;

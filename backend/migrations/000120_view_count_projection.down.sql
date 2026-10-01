-- Back to an API-maintained view count: the code before 000120 increments posts.view_count
-- itself after inserting a view row.
DROP TRIGGER IF EXISTS post_views_count_insert ON post_views;
DROP TRIGGER IF EXISTS post_views_count_update ON post_views;
DROP TRIGGER IF EXISTS post_views_count_delete ON post_views;
DROP FUNCTION IF EXISTS post_views_count_projection();
DROP FUNCTION IF EXISTS rebuild_view_counts(uuid);
DROP FUNCTION IF EXISTS view_count_drift(uuid);

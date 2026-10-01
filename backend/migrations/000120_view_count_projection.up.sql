-- A post's view count is a projection of its view rows (idx 77: counters and projections
-- are rebuildable from authoritative records). post_views is the record, one row per
-- (post, viewer type, viewer id); the counter is derived from it:
--
--   posts.view_count = the post's post_views rows
--
-- The API used to insert the view row and then increment the counter in a second statement
-- outside any transaction, so a caller that left right after the insert (a disconnect, a
-- deadline) stored the view but never counted it, and the viewer's every retry hit the
-- unique key and counted nothing again (measured, idx 77 slice 4; the restored production
-- dump has one such post: view_count 0 with one view row). This trigger moves the counter
-- inside the view row write's own transaction instead: a committed view counts once, a
-- rolled-back one never, a repeated view that inserts nothing moves nothing, and a deleted
-- or moved view row is uncounted where it was. Whoever writes the view rows (the API, a
-- fixture, an operator, a cleanup) keeps the count consistent.
--
-- Blog posts are not covered: blog_posts.view_count has no per-view record to derive it
-- from, so that counter is itself the record.
--
-- REBUILD PATH (operator, READ COMMITTED):
--   SELECT * FROM view_count_drift();                   -- posts whose stored count differs
--   SELECT * FROM view_count_drift('<post uuid>');      -- one post
--   SELECT rebuild_view_counts();                       -- repair every post
--   SELECT rebuild_view_counts('<post uuid>');          -- repair one post
-- The rebuild locks the posts' rows before it recounts. A view's trigger takes the same row
-- lock, so a view in flight is counted by the rebuild once it commits, and a view that
-- arrives during the rebuild waits and then lands on the rebuilt value. A consistent row is
-- not rewritten; a second rebuild returns 0.

CREATE OR REPLACE FUNCTION post_views_count_projection() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        UPDATE posts SET view_count = GREATEST(view_count - 1, 0) WHERE id = OLD.post_id;
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
        UPDATE posts SET view_count = view_count + 1 WHERE id = NEW.post_id;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS post_views_count_insert ON post_views;
CREATE TRIGGER post_views_count_insert
    AFTER INSERT ON post_views
    FOR EACH ROW
    EXECUTE FUNCTION post_views_count_projection();

DROP TRIGGER IF EXISTS post_views_count_update ON post_views;
CREATE TRIGGER post_views_count_update
    AFTER UPDATE OF post_id ON post_views
    FOR EACH ROW
    WHEN (OLD.post_id IS DISTINCT FROM NEW.post_id)
    EXECUTE FUNCTION post_views_count_projection();

DROP TRIGGER IF EXISTS post_views_count_delete ON post_views;
CREATE TRIGGER post_views_count_delete
    AFTER DELETE ON post_views
    FOR EACH ROW
    EXECUTE FUNCTION post_views_count_projection();

-- Read-only reconciliation: the posts (all, or one) whose stored view count differs from
-- their view rows, with both values.
CREATE OR REPLACE FUNCTION view_count_drift(p_post_id uuid DEFAULT NULL)
RETURNS TABLE (post_id uuid, stored_view_count integer, view_rows integer)
LANGUAGE sql STABLE AS $$
    SELECT p.id, p.view_count, v.n
    FROM posts p
    CROSS JOIN LATERAL (
        SELECT COUNT(*)::integer AS n FROM post_views pv WHERE pv.post_id = p.id
    ) v
    WHERE (p_post_id IS NULL OR p.id = p_post_id)
      AND p.view_count IS DISTINCT FROM v.n
    ORDER BY p.id;
$$;

CREATE OR REPLACE FUNCTION rebuild_view_counts(p_post_id uuid DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    repaired integer;
BEGIN
    -- Lock first, recount after: the UPDATE below runs on a snapshot taken once every
    -- in-flight view on these posts has committed, and new ones wait for us.
    PERFORM 1 FROM posts
     WHERE p_post_id IS NULL OR id = p_post_id
     ORDER BY id
       FOR NO KEY UPDATE;
    UPDATE posts p
       SET view_count = d.view_rows
      FROM view_count_drift(p_post_id) d
     WHERE p.id = d.post_id;
    GET DIAGNOSTICS repaired = ROW_COUNT;
    RETURN repaired;
END;
$$;

COMMENT ON FUNCTION view_count_drift(uuid) IS
    'Posts whose stored view_count differs from their post_views rows (000120). Read-only.';
COMMENT ON FUNCTION rebuild_view_counts(uuid) IS
    'Recompute posts.view_count from post_views (all posts when NULL); returns posts repaired (000120).';

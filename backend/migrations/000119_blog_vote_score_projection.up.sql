-- Blog post scores join the vote score projection of 000113 (idx 77: counters and
-- projections are rebuildable from authoritative records). votes stays the record:
--
--   blog_posts.upvotes / blog_posts.downvotes = confirmed 'blog_post' votes, by direction
--
-- BlogPostRepository.Vote had the shape 000113 removed from posts and replies: it read the
-- voter's previous vote OUTSIDE its transaction, then moved the counters itself. Five rounds
-- of "up, then eight concurrent downs" from one voter left a blog post at upvotes = -34,
-- downvotes = 36 while its votes said 1 and 1, and one of eight concurrent first votes failed
-- on votes_unique_per_target (measured, idx 77 slice 3). The 000113 triggers now move the blog
-- counters too, inside the vote write's own transaction, and the API writes one upsert.
--
-- This redefines 000113's three functions and triggers in place; every 000113 rule holds for
-- blog posts unchanged (confirmed votes only, NULL counted as 0, clamped at 0, a retarget is
-- not a vote event, the rebuild locks its rows before it recounts).
--
-- REBUILD PATH (operator, READ COMMITTED), now over posts, replies and blog posts:
--   SELECT * FROM vote_score_drift();                        -- every target whose score differs
--   SELECT * FROM vote_score_drift('blog_post');             -- only blog posts
--   SELECT rebuild_vote_scores();                            -- repair every target
--   SELECT rebuild_vote_scores('blog_post', '<blog uuid>');  -- repair one blog post
-- A full rebuild locks posts, then replies, then blog posts. A vote locks only its own
-- target's row, so no new lock order is introduced. The legacy contribution target types
-- (answer, response, approach) keep their own counters and are refused by the rebuild.

CREATE OR REPLACE FUNCTION votes_score_projection() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    d_up   integer := 0;
    d_down integer := 0;
    t_type text;
    t_id   uuid;
BEGIN
    IF TG_OP <> 'INSERT' AND OLD.confirmed IS TRUE THEN
        IF OLD.direction = 'up' THEN d_up := d_up - 1; ELSE d_down := d_down - 1; END IF;
    END IF;
    IF TG_OP <> 'DELETE' AND NEW.confirmed IS TRUE THEN
        IF NEW.direction = 'up' THEN d_up := d_up + 1; ELSE d_down := d_down + 1; END IF;
    END IF;
    IF d_up = 0 AND d_down = 0 THEN
        RETURN NULL;
    END IF;

    -- An UPDATE never changes the target here (the trigger's WHEN clause).
    IF TG_OP = 'DELETE' THEN
        t_type := OLD.target_type; t_id := OLD.target_id;
    ELSE
        t_type := NEW.target_type; t_id := NEW.target_id;
    END IF;

    IF t_type = 'post' THEN
        UPDATE posts
           SET upvotes   = GREATEST(COALESCE(upvotes, 0) + d_up, 0),
               downvotes = GREATEST(COALESCE(downvotes, 0) + d_down, 0)
         WHERE id = t_id;
    ELSIF t_type = 'blog_post' THEN
        UPDATE blog_posts
           SET upvotes   = GREATEST(COALESCE(upvotes, 0) + d_up, 0),
               downvotes = GREATEST(COALESCE(downvotes, 0) + d_down, 0)
         WHERE id = t_id;
    ELSE
        UPDATE replies
           SET upvotes   = GREATEST(upvotes + d_up, 0),
               downvotes = GREATEST(downvotes + d_down, 0)
         WHERE id = t_id;
    END IF;
    RETURN NULL;
END;
$$;

DROP TRIGGER IF EXISTS votes_score_insert ON votes;
CREATE TRIGGER votes_score_insert
    AFTER INSERT ON votes
    FOR EACH ROW
    WHEN (NEW.target_type IN ('post', 'reply', 'blog_post') AND NEW.confirmed IS TRUE)
    EXECUTE FUNCTION votes_score_projection();

DROP TRIGGER IF EXISTS votes_score_update ON votes;
CREATE TRIGGER votes_score_update
    AFTER UPDATE OF direction, confirmed ON votes
    FOR EACH ROW
    WHEN (NEW.target_type IN ('post', 'reply', 'blog_post')
          AND OLD.target_type = NEW.target_type AND OLD.target_id = NEW.target_id
          AND (OLD.direction IS DISTINCT FROM NEW.direction
               OR OLD.confirmed IS DISTINCT FROM NEW.confirmed))
    EXECUTE FUNCTION votes_score_projection();

DROP TRIGGER IF EXISTS votes_score_delete ON votes;
CREATE TRIGGER votes_score_delete
    AFTER DELETE ON votes
    FOR EACH ROW
    WHEN (OLD.target_type IN ('post', 'reply', 'blog_post') AND OLD.confirmed IS TRUE)
    EXECUTE FUNCTION votes_score_projection();

-- Read-only reconciliation: the posts, replies and blog posts (filtered by target type and/or
-- id) whose stored score differs from their confirmed votes, with both values. A NULL counter
-- differs.
CREATE OR REPLACE FUNCTION vote_score_drift(p_target_type text DEFAULT NULL, p_target_id uuid DEFAULT NULL)
RETURNS TABLE (target_type text, target_id uuid,
               stored_upvotes integer, vote_upvotes integer,
               stored_downvotes integer, vote_downvotes integer)
LANGUAGE sql STABLE AS $$
    SELECT 'post'::text, p.id, p.upvotes, t.up, p.downvotes, t.down
    FROM posts p
    CROSS JOIN LATERAL (
        SELECT COUNT(*) FILTER (WHERE v.direction = 'up')::integer   AS up,
               COUNT(*) FILTER (WHERE v.direction = 'down')::integer AS down
        FROM votes v
        WHERE v.target_type = 'post' AND v.target_id = p.id AND v.confirmed = true
    ) t
    WHERE (p_target_type IS NULL OR p_target_type = 'post')
      AND (p_target_id IS NULL OR p.id = p_target_id)
      AND (p.upvotes IS DISTINCT FROM t.up OR p.downvotes IS DISTINCT FROM t.down)
    UNION ALL
    SELECT 'reply'::text, r.id, r.upvotes, t.up, r.downvotes, t.down
    FROM replies r
    CROSS JOIN LATERAL (
        SELECT COUNT(*) FILTER (WHERE v.direction = 'up')::integer   AS up,
               COUNT(*) FILTER (WHERE v.direction = 'down')::integer AS down
        FROM votes v
        WHERE v.target_type = 'reply' AND v.target_id = r.id AND v.confirmed = true
    ) t
    WHERE (p_target_type IS NULL OR p_target_type = 'reply')
      AND (p_target_id IS NULL OR r.id = p_target_id)
      AND (r.upvotes IS DISTINCT FROM t.up OR r.downvotes IS DISTINCT FROM t.down)
    UNION ALL
    SELECT 'blog_post'::text, b.id, b.upvotes, t.up, b.downvotes, t.down
    FROM blog_posts b
    CROSS JOIN LATERAL (
        SELECT COUNT(*) FILTER (WHERE v.direction = 'up')::integer   AS up,
               COUNT(*) FILTER (WHERE v.direction = 'down')::integer AS down
        FROM votes v
        WHERE v.target_type = 'blog_post' AND v.target_id = b.id AND v.confirmed = true
    ) t
    WHERE (p_target_type IS NULL OR p_target_type = 'blog_post')
      AND (p_target_id IS NULL OR b.id = p_target_id)
      AND (b.upvotes IS DISTINCT FROM t.up OR b.downvotes IS DISTINCT FROM t.down)
    ORDER BY 1, 2;
$$;

CREATE OR REPLACE FUNCTION rebuild_vote_scores(p_target_type text DEFAULT NULL, p_target_id uuid DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    repaired integer := 0;
    n        integer;
BEGIN
    IF p_target_type IS NOT NULL AND p_target_type NOT IN ('post', 'reply', 'blog_post') THEN
        RAISE EXCEPTION 'vote scores are projected for post, reply and blog_post targets, not %', p_target_type;
    END IF;

    -- Lock first, recount after: each UPDATE below runs on a snapshot taken once every
    -- in-flight vote on these rows has committed, and new ones wait for us.
    IF p_target_type IS NULL OR p_target_type = 'post' THEN
        PERFORM 1 FROM posts
         WHERE p_target_id IS NULL OR id = p_target_id
         ORDER BY id
           FOR NO KEY UPDATE;
        UPDATE posts p
           SET upvotes = d.vote_upvotes, downvotes = d.vote_downvotes
          FROM vote_score_drift('post', p_target_id) d
         WHERE p.id = d.target_id;
        GET DIAGNOSTICS n = ROW_COUNT;
        repaired := repaired + n;
    END IF;

    IF p_target_type IS NULL OR p_target_type = 'reply' THEN
        PERFORM 1 FROM replies
         WHERE p_target_id IS NULL OR id = p_target_id
         ORDER BY id
           FOR NO KEY UPDATE;
        UPDATE replies r
           SET upvotes = d.vote_upvotes, downvotes = d.vote_downvotes
          FROM vote_score_drift('reply', p_target_id) d
         WHERE r.id = d.target_id;
        GET DIAGNOSTICS n = ROW_COUNT;
        repaired := repaired + n;
    END IF;

    IF p_target_type IS NULL OR p_target_type = 'blog_post' THEN
        PERFORM 1 FROM blog_posts
         WHERE p_target_id IS NULL OR id = p_target_id
         ORDER BY id
           FOR NO KEY UPDATE;
        UPDATE blog_posts b
           SET upvotes = d.vote_upvotes, downvotes = d.vote_downvotes
          FROM vote_score_drift('blog_post', p_target_id) d
         WHERE b.id = d.target_id;
        GET DIAGNOSTICS n = ROW_COUNT;
        repaired := repaired + n;
    END IF;
    RETURN repaired;
END;
$$;

COMMENT ON FUNCTION vote_score_drift(text, uuid) IS
    'Posts/replies/blog posts whose stored upvotes/downvotes differ from their confirmed votes (000113, 000119). Read-only.';
COMMENT ON FUNCTION rebuild_vote_scores(text, uuid) IS
    'Recompute posts/replies/blog posts upvotes/downvotes from confirmed votes (all when NULL); returns targets repaired (000113, 000119).';

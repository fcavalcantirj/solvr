-- Post and reply scores are a projection of their votes (idx 77: counters and projections
-- are rebuildable from authoritative records). votes is the record; the counters are derived
-- from it, counting confirmed votes only (SPEC: "server-computed from confirmed votes"):
--
--   posts.upvotes / posts.downvotes     = confirmed 'post' votes on the post, by direction
--   replies.upvotes / replies.downvotes = confirmed 'reply' votes on the reply, by direction
--
-- The API used to read the voter's previous vote OUTSIDE its transaction and then add or
-- move one vote in the counters itself, so concurrent requests from one voter (a double
-- click, a client retry) each moved the counters again: eight concurrent flips from up to
-- down drove a post to upvotes = -26 while its votes said 1 (measured, idx 77 slice 2).
-- These triggers move the counters inside the vote write's own transaction instead, from
-- the row change itself: a committed vote counts once, a rolled-back one never, and a
-- replayed vote that changes nothing moves nothing. Whoever writes the vote row (the API,
-- a fixture, an operator) keeps the score consistent, and un-/re-confirming a vote moves it.
--
-- Not a vote event: a statement that retargets a vote (target_type/target_id). The
-- contribution cutover (RemapContributionVotesAndReports) retargets legacy votes onto
-- replies whose counters MigrateContributions already copied from the legacy rows, so
-- counting the move would count those votes twice. Run the rebuild after a retarget.
--
-- REBUILD PATH (operator, READ COMMITTED):
--   SELECT * FROM vote_score_drift();                        -- targets whose stored score differs
--   SELECT * FROM vote_score_drift('reply');                 -- only replies ('post' for posts)
--   SELECT rebuild_vote_scores();                            -- repair every post and reply
--   SELECT rebuild_vote_scores('post', '<post uuid>');       -- repair one target
-- The rebuild locks the targets' rows before it recounts. A vote's trigger takes the same
-- row lock, so a vote in flight is counted by the rebuild once it commits, and a vote that
-- arrives during the rebuild waits and then lands on the rebuilt value. A consistent row is
-- not rewritten; a second rebuild returns 0. Other vote target types (blog_post and the
-- legacy contribution types) keep their own counters and are not covered here.

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
    WHEN (NEW.target_type IN ('post', 'reply') AND NEW.confirmed IS TRUE)
    EXECUTE FUNCTION votes_score_projection();

DROP TRIGGER IF EXISTS votes_score_update ON votes;
CREATE TRIGGER votes_score_update
    AFTER UPDATE OF direction, confirmed ON votes
    FOR EACH ROW
    WHEN (NEW.target_type IN ('post', 'reply')
          AND OLD.target_type = NEW.target_type AND OLD.target_id = NEW.target_id
          AND (OLD.direction IS DISTINCT FROM NEW.direction
               OR OLD.confirmed IS DISTINCT FROM NEW.confirmed))
    EXECUTE FUNCTION votes_score_projection();

DROP TRIGGER IF EXISTS votes_score_delete ON votes;
CREATE TRIGGER votes_score_delete
    AFTER DELETE ON votes
    FOR EACH ROW
    WHEN (OLD.target_type IN ('post', 'reply') AND OLD.confirmed IS TRUE)
    EXECUTE FUNCTION votes_score_projection();

-- Read-only reconciliation: the posts and replies (filtered by target type and/or id) whose
-- stored score differs from their confirmed votes, with both values. A NULL counter differs.
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
    ORDER BY 1, 2;
$$;

CREATE OR REPLACE FUNCTION rebuild_vote_scores(p_target_type text DEFAULT NULL, p_target_id uuid DEFAULT NULL)
RETURNS integer
LANGUAGE plpgsql AS $$
DECLARE
    repaired integer := 0;
    n        integer;
BEGIN
    IF p_target_type IS NOT NULL AND p_target_type NOT IN ('post', 'reply') THEN
        RAISE EXCEPTION 'vote scores are projected for post and reply targets, not %', p_target_type;
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
    RETURN repaired;
END;
$$;

COMMENT ON FUNCTION vote_score_drift(text, uuid) IS
    'Posts/replies whose stored upvotes/downvotes differ from their confirmed votes (000113). Read-only.';
COMMENT ON FUNCTION rebuild_vote_scores(text, uuid) IS
    'Recompute posts/replies upvotes/downvotes from confirmed votes (all when NULL); returns targets repaired (000113).';

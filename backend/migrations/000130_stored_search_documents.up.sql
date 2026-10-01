-- Keyword search reads stored documents (idx 77 step 1: search indexes from measured query
-- plans). Until now the keyword documents were GIN expression indexes over
-- to_tsvector('english', title || ' ' || description) and to_tsvector('english', body): the
-- index finds the rows, but ranking them (ts_rank / ts_rank_cd) and rechecking them parsed every
-- matching document again, twice per row.
--
-- Measured before this migration (idx 77 slice 18 spike: 200k posts of ~580 characters, the
-- production median is 606; 1M replies of ~220):
--   a term in every post, default hybrid search (hybrid_search)  24.3 s, of it 23.4 s a scan
--                                                                parsing 191,941 posts
--   the same term, keyword fallback (no query embedding)         26.2 s
--   a term in 10% of the posts, hybrid / keyword                 2.1 s / 3.0 s
--   ranking the 191,941 matches from a stored tsvector           141 ms (8.1 s from the
--                                                                expression, 3 parallel workers)
--   a term in 10%, ranked from the stored tsvector               131 ms (592 ms)
--
-- The documents become STORED generated columns. Postgres computes them in the row's own write
-- from the row's own text, like the expression indexes did, so they cannot drift from the text
-- and have no rebuild path to run (REINDEX of their GIN indexes at most). The expression
-- indexes they replace are dropped; every reader moves to the columns: the keyword queries in
-- internal/db (search.go, search_canonical.go, search_replies.go) and the two hybrid functions
-- below, whose definitions are otherwise those of 000080 and 000110.
--
-- Lock: ADD COLUMN ... STORED rewrites posts and replies under ACCESS EXCLUSIVE (632 posts and
-- 1,018 replies in the post-purge rehearsal copy at 113: milliseconds).

ALTER TABLE posts ADD COLUMN search_document tsvector
    GENERATED ALWAYS AS (to_tsvector('english', title || ' ' || description)) STORED;
CREATE INDEX idx_posts_search_document ON posts USING GIN (search_document);
DROP INDEX IF EXISTS idx_posts_search;

ALTER TABLE replies ADD COLUMN search_document tsvector
    GENERATED ALWAYS AS (to_tsvector('english', body)) STORED;
CREATE INDEX idx_replies_search_document ON replies USING GIN (search_document) WHERE deleted_at IS NULL;
DROP INDEX IF EXISTS idx_replies_body_tsvector;

CREATE OR REPLACE FUNCTION hybrid_search(
    query_text text,
    query_embedding vector(1024),
    match_count int DEFAULT 20,
    fts_weight float DEFAULT 1.0,
    vec_weight float DEFAULT 1.0,
    rrf_k int DEFAULT 60,
    viewer_human uuid DEFAULT NULL
)
RETURNS TABLE(post_id uuid, rrf_score float8)
LANGUAGE sql STABLE
AS $$
    WITH full_text AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY ts_rank_cd(search_document, to_tsquery('english', query_text)) DESC
               ) AS rank_ix
        FROM posts
        WHERE deleted_at IS NULL
          AND status NOT IN ('pending_review', 'rejected', 'draft')
          AND (visibility = 'public' OR (viewer_human IS NOT NULL AND owner_human_id = viewer_human))
          AND search_document @@ to_tsquery('english', query_text)
        LIMIT match_count * 2
    ),
    semantic AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY embedding <=> query_embedding
               ) AS rank_ix
        FROM posts
        WHERE deleted_at IS NULL
          AND status NOT IN ('pending_review', 'rejected', 'draft')
          AND (visibility = 'public' OR (viewer_human IS NOT NULL AND owner_human_id = viewer_human))
          AND embedding IS NOT NULL
          AND embedding <=> query_embedding < 0.85
        ORDER BY embedding <=> query_embedding
        LIMIT match_count * 2
    )
    SELECT COALESCE(ft.id, s.id) AS post_id,
           COALESCE(1.0 / (rrf_k + ft.rank_ix), 0.0) * fts_weight
           + COALESCE(1.0 / (rrf_k + s.rank_ix), 0.0) * vec_weight AS rrf_score
    FROM full_text ft
    FULL OUTER JOIN semantic s ON ft.id = s.id
    ORDER BY rrf_score DESC
    LIMIT match_count;
$$;

CREATE OR REPLACE FUNCTION hybrid_search_replies(
    query_text text,
    query_embedding vector(1024),
    match_count int DEFAULT 20,
    fts_weight float DEFAULT 1.0,
    vec_weight float DEFAULT 1.0,
    rrf_k int DEFAULT 60,
    viewer_human uuid DEFAULT NULL
)
RETURNS TABLE(reply_id uuid, post_id uuid, rrf_score float8)
LANGUAGE sql STABLE
AS $$
    WITH eligible AS (
        SELECT r.id, r.post_id, r.search_document, r.embedding
        FROM replies r
        JOIN posts p ON p.id = r.post_id
        WHERE r.deleted_at IS NULL
          AND p.deleted_at IS NULL
          AND p.publication_state = 'published'
          AND p.moderation_state = 'approved'
          AND (p.visibility = 'public' OR (viewer_human IS NOT NULL AND p.owner_human_id = viewer_human))
    ),
    full_text AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY ts_rank_cd(search_document, to_tsquery('english', query_text)) DESC
               ) AS rank_ix
        FROM eligible
        WHERE search_document @@ to_tsquery('english', query_text)
        LIMIT match_count * 2
    ),
    semantic AS (
        SELECT id,
               ROW_NUMBER() OVER (ORDER BY embedding <=> query_embedding) AS rank_ix
        FROM eligible
        WHERE embedding IS NOT NULL
          AND embedding <=> query_embedding < 0.85
        ORDER BY embedding <=> query_embedding
        LIMIT match_count * 2
    )
    SELECT e.id AS reply_id,
           e.post_id,
           COALESCE(1.0 / (rrf_k + ft.rank_ix), 0.0) * fts_weight
           + COALESCE(1.0 / (rrf_k + s.rank_ix), 0.0) * vec_weight AS rrf_score
    FROM full_text ft
    FULL OUTER JOIN semantic s ON ft.id = s.id
    JOIN eligible e ON e.id = COALESCE(ft.id, s.id)
    ORDER BY rrf_score DESC
    LIMIT match_count;
$$;

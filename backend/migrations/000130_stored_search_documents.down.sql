-- Back to the expression documents of 000003 / 000110: the hybrid functions as 000080 and
-- 000110 defined them, the expression GIN indexes, and no stored columns.
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
                   ORDER BY ts_rank_cd(
                       to_tsvector('english', title || ' ' || description),
                       to_tsquery('english', query_text)
                   ) DESC
               ) AS rank_ix
        FROM posts
        WHERE deleted_at IS NULL
          AND status NOT IN ('pending_review', 'rejected', 'draft')
          AND (visibility = 'public' OR (viewer_human IS NOT NULL AND owner_human_id = viewer_human))
          AND to_tsvector('english', title || ' ' || description) @@ to_tsquery('english', query_text)
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
        SELECT r.id, r.post_id, r.body, r.embedding
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
                   ORDER BY ts_rank_cd(to_tsvector('english', body), to_tsquery('english', query_text)) DESC
               ) AS rank_ix
        FROM eligible
        WHERE to_tsvector('english', body) @@ to_tsquery('english', query_text)
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

CREATE INDEX IF NOT EXISTS idx_posts_search ON posts
    USING GIN(to_tsvector('english', title || ' ' || description));
CREATE INDEX IF NOT EXISTS idx_replies_body_tsvector
    ON replies USING GIN (to_tsvector('english', body)) WHERE deleted_at IS NULL;

DROP INDEX IF EXISTS idx_replies_search_document;
ALTER TABLE replies DROP COLUMN IF EXISTS search_document;
DROP INDEX IF EXISTS idx_posts_search_document;
ALTER TABLE posts DROP COLUMN IF EXISTS search_document;

-- Revert BART-151 post visibility. Restore the 000071 hybrid_search (6-arg, no viewer filter).
DROP FUNCTION IF EXISTS hybrid_search(text, vector(1024), int, float, float, int, uuid);

CREATE FUNCTION hybrid_search(
    query_text text,
    query_embedding vector(1024),
    match_count int DEFAULT 20,
    fts_weight float DEFAULT 1.0,
    vec_weight float DEFAULT 1.0,
    rrf_k int DEFAULT 60
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

DROP INDEX IF EXISTS idx_posts_owner_human;
ALTER TABLE posts DROP COLUMN IF EXISTS owner_human_id;
ALTER TABLE posts DROP COLUMN IF EXISTS visibility;

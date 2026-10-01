-- Restores hybrid_search_replies as 000130 defined it (one materialized eligible CTE, no tie break).

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

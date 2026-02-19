-- Hybrid search function for posts table
-- Combines full-text search (keyword matching) with vector similarity (semantic matching)
-- using Reciprocal Rank Fusion (RRF) from Cormack et al. SIGIR 2009
CREATE OR REPLACE FUNCTION hybrid_search(
    query_text text,
    query_embedding vector(1024),
    match_count int DEFAULT 20,
    fts_weight float DEFAULT 1.0,
    vec_weight float DEFAULT 1.0,
    rrf_k int DEFAULT 60
)
RETURNS SETOF posts
LANGUAGE sql STABLE
AS $$
    WITH full_text AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY ts_rank_cd(
                       to_tsvector('english', title || ' ' || description),
                       websearch_to_tsquery('english', query_text)
                   ) DESC
               ) AS rank_ix
        FROM posts
        WHERE deleted_at IS NULL
          AND to_tsvector('english', title || ' ' || description) @@ websearch_to_tsquery('english', query_text)
        LIMIT match_count * 2
    ),
    semantic AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY embedding <=> query_embedding
               ) AS rank_ix
        FROM posts
        WHERE deleted_at IS NULL
          AND embedding IS NOT NULL
        ORDER BY embedding <=> query_embedding
        LIMIT match_count * 2
    )
    SELECT p.*
    FROM (
        SELECT COALESCE(ft.id, s.id) AS id,
               COALESCE(1.0 / (rrf_k + ft.rank_ix), 0.0) * fts_weight
               + COALESCE(1.0 / (rrf_k + s.rank_ix), 0.0) * vec_weight AS score
        FROM full_text ft
        FULL OUTER JOIN semantic s ON ft.id = s.id
        ORDER BY score DESC
        LIMIT match_count
    ) ranked
    JOIN posts p ON p.id = ranked.id
    ORDER BY ranked.score DESC;
$$;

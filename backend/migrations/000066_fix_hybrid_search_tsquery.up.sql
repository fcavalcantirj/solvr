-- Fix hybrid_search to use to_tsquery instead of websearch_to_tsquery.
--
-- websearch_to_tsquery uses AND logic (all words must match), but the Go code
-- builds tsquery strings with OR prefix logic (word1:* | word2:*).
-- This mismatch caused 0 FTS results for multi-word queries like "golang error",
-- making search return only semantically similar but keyword-irrelevant posts.
--
-- Also adds a semantic distance threshold (< 1.0) to prevent completely
-- unrelated posts from appearing in results.

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
          AND embedding <=> query_embedding < 1.0
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

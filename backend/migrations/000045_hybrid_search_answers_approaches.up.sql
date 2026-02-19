-- Hybrid search function for answers table
-- Combines full-text search (keyword matching) with vector similarity (semantic matching)
-- using Reciprocal Rank Fusion (RRF) from Cormack et al. SIGIR 2009
CREATE OR REPLACE FUNCTION hybrid_search_answers(
    query_text text,
    query_embedding vector(1024),
    match_count int DEFAULT 20,
    fts_weight float DEFAULT 1.0,
    vec_weight float DEFAULT 1.0,
    rrf_k int DEFAULT 60
)
RETURNS SETOF answers
LANGUAGE sql STABLE
AS $$
    WITH full_text AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY ts_rank_cd(
                       to_tsvector('english', content),
                       websearch_to_tsquery('english', query_text)
                   ) DESC
               ) AS rank_ix
        FROM answers
        WHERE deleted_at IS NULL
          AND to_tsvector('english', content) @@ websearch_to_tsquery('english', query_text)
        LIMIT match_count * 2
    ),
    semantic AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY embedding <=> query_embedding
               ) AS rank_ix
        FROM answers
        WHERE deleted_at IS NULL
          AND embedding IS NOT NULL
        ORDER BY embedding <=> query_embedding
        LIMIT match_count * 2
    )
    SELECT a.*
    FROM (
        SELECT COALESCE(ft.id, s.id) AS id,
               COALESCE(1.0 / (rrf_k + ft.rank_ix), 0.0) * fts_weight
               + COALESCE(1.0 / (rrf_k + s.rank_ix), 0.0) * vec_weight AS score
        FROM full_text ft
        FULL OUTER JOIN semantic s ON ft.id = s.id
        ORDER BY score DESC
        LIMIT match_count
    ) ranked
    JOIN answers a ON a.id = ranked.id
    ORDER BY ranked.score DESC;
$$;

-- Hybrid search function for approaches table
-- Combines full-text search on angle, method, outcome, and solution
-- with vector similarity on the embedding column
CREATE OR REPLACE FUNCTION hybrid_search_approaches(
    query_text text,
    query_embedding vector(1024),
    match_count int DEFAULT 20,
    fts_weight float DEFAULT 1.0,
    vec_weight float DEFAULT 1.0,
    rrf_k int DEFAULT 60
)
RETURNS SETOF approaches
LANGUAGE sql STABLE
AS $$
    WITH full_text AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY ts_rank_cd(
                       to_tsvector('english',
                           COALESCE(angle, '') || ' ' ||
                           COALESCE(method, '') || ' ' ||
                           COALESCE(outcome, '') || ' ' ||
                           COALESCE(solution, '')
                       ),
                       websearch_to_tsquery('english', query_text)
                   ) DESC
               ) AS rank_ix
        FROM approaches
        WHERE deleted_at IS NULL
          AND to_tsvector('english',
              COALESCE(angle, '') || ' ' ||
              COALESCE(method, '') || ' ' ||
              COALESCE(outcome, '') || ' ' ||
              COALESCE(solution, '')
          ) @@ websearch_to_tsquery('english', query_text)
        LIMIT match_count * 2
    ),
    semantic AS (
        SELECT id,
               ROW_NUMBER() OVER (
                   ORDER BY embedding <=> query_embedding
               ) AS rank_ix
        FROM approaches
        WHERE deleted_at IS NULL
          AND embedding IS NOT NULL
        ORDER BY embedding <=> query_embedding
        LIMIT match_count * 2
    )
    SELECT a.*
    FROM (
        SELECT COALESCE(ft.id, s.id) AS id,
               COALESCE(1.0 / (rrf_k + ft.rank_ix), 0.0) * fts_weight
               + COALESCE(1.0 / (rrf_k + s.rank_ix), 0.0) * vec_weight AS score
        FROM full_text ft
        FULL OUTER JOIN semantic s ON ft.id = s.id
        ORDER BY score DESC
        LIMIT match_count
    ) ranked
    JOIN approaches a ON a.id = ranked.id
    ORDER BY ranked.score DESC;
$$;

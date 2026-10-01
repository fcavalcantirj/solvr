-- hybrid_search_replies reads its candidates through the indexes (idx 77 step 1: search indexes
-- from measured query plans). Until now one CTE, eligible, held every live reply of every
-- visible published post, and it was read three times (keyword candidates, vector candidates,
-- the final rows), so Postgres materialized it: each call copied every eligible reply with its
-- document and embedding, then filtered the copy. Neither idx_replies_search_document nor
-- idx_replies_embedding could serve it.
--
-- Measured (idx 77 slice 19 spike: 200k posts, 1M replies of ~220 characters, a tenth of the
-- posts and of the replies embedded), this function's statement inside the default hybrid
-- search, before -> after:
--   a term in 1 in 2,000 replies    3,219 ms -> 5 ms
--   a term in 1 in 50 replies       3,896 ms -> 95 ms
--   a term in 1 in 5 replies        4,632 ms -> 917 ms (the 195k matches are read and ranked)
-- Before, 1.0 s of it built the 935k-row CTE and up to 3.8 s computed the distance of every
-- embedded eligible reply.
--
-- Each candidate list now joins replies to posts itself, the shape of hybrid_search: keyword
-- candidates come from the GIN index on the stored document (or a scan when a term is in a large
-- share of the replies), vector candidates in distance order from the planner's choice (the HNSW
-- index on a large table, a scan on a small one), and the final rows join replies by id. Through
-- the HNSW index the vector candidates are approximate and at most hnsw.ef_search (40) of them,
-- as hybrid_search's already are for posts; the spike's 100k embedded replies gave 32 where the
-- exact scan gave 120. Ties in either rank are broken by reply id, so a call returns
-- the same rows every time; before, which of several equally ranked replies made the cut was the
-- scan's order. On the production copy at 113 (1,018 replies, 285 embedded; 40 queries built
-- from its posts' embeddings and titles) the planner scans for the vector candidates, and this
-- shape and the old one, both with the tie break, return the same 2,400 rows and scores.

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
    WITH full_text AS (
        SELECT r.id,
               ROW_NUMBER() OVER (
                   ORDER BY ts_rank_cd(r.search_document, to_tsquery('english', query_text)) DESC, r.id
               ) AS rank_ix
        FROM replies r
        JOIN posts p ON p.id = r.post_id
        WHERE r.deleted_at IS NULL
          AND p.deleted_at IS NULL
          AND p.publication_state = 'published'
          AND p.moderation_state = 'approved'
          AND (p.visibility = 'public' OR (viewer_human IS NOT NULL AND p.owner_human_id = viewer_human))
          AND r.search_document @@ to_tsquery('english', query_text)
        ORDER BY rank_ix
        LIMIT match_count * 2
    ),
    semantic AS (
        SELECT r.id,
               ROW_NUMBER() OVER (ORDER BY r.embedding <=> query_embedding, r.id) AS rank_ix
        FROM replies r
        JOIN posts p ON p.id = r.post_id
        WHERE r.deleted_at IS NULL
          AND p.deleted_at IS NULL
          AND p.publication_state = 'published'
          AND p.moderation_state = 'approved'
          AND (p.visibility = 'public' OR (viewer_human IS NOT NULL AND p.owner_human_id = viewer_human))
          AND r.embedding IS NOT NULL
          AND r.embedding <=> query_embedding < 0.85
        ORDER BY r.embedding <=> query_embedding, r.id
        LIMIT match_count * 2
    )
    SELECT r.id AS reply_id,
           r.post_id,
           COALESCE(1.0 / (rrf_k + ft.rank_ix), 0.0) * fts_weight
           + COALESCE(1.0 / (rrf_k + s.rank_ix), 0.0) * vec_weight AS rrf_score
    FROM full_text ft
    FULL OUTER JOIN semantic s ON ft.id = s.id
    JOIN replies r ON r.id = COALESCE(ft.id, s.id)
    ORDER BY rrf_score DESC, r.id
    LIMIT match_count;
$$;

-- BART-151: family-scoped visibility tier on KB posts (public | family).
-- A 'family' post is visible only to its owner's family: the human + all agents
-- sharing agents.human_id (owner_human_id == agents.human_id). Existing rows default
-- to 'public' — backwards compatible; privacy is strictly opt-in.

ALTER TABLE posts ADD COLUMN IF NOT EXISTS visibility VARCHAR(20) NOT NULL DEFAULT 'public'
    CHECK (visibility IN ('public', 'family'));

ALTER TABLE posts ADD COLUMN IF NOT EXISTS owner_human_id UUID REFERENCES users(id) ON DELETE SET NULL;

-- Partial index for owner-scoped "my family posts" lookups.
CREATE INDEX IF NOT EXISTS idx_posts_owner_human ON posts (owner_human_id) WHERE visibility = 'family';

-- Replace hybrid_search() to exclude family posts unless the viewer's human owns them.
-- New trailing param viewer_human uuid (DEFAULT NULL = anonymous/cross-family -> public-only).
-- Must DROP the old 6-arg signature first (return type unchanged, but avoid overload ambiguity
-- so a 6-arg positional call still resolves to this function).
DROP FUNCTION IF EXISTS hybrid_search(text, vector(1024), int, float, float, int);

CREATE FUNCTION hybrid_search(
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

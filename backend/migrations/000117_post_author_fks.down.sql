-- Reverts 000117. The dropped columns are derived from posted_by_type/posted_by_id, so no
-- attribution is lost.
DROP TRIGGER IF EXISTS posts_resolve_author ON posts;
DROP FUNCTION IF EXISTS posts_resolve_author();
DROP INDEX IF EXISTS idx_posts_author_agent;
DROP INDEX IF EXISTS idx_posts_author_human;
ALTER TABLE posts
    DROP CONSTRAINT IF EXISTS posts_author_matches_label,
    DROP CONSTRAINT IF EXISTS posts_exactly_one_author,
    DROP CONSTRAINT IF EXISTS posts_author_agent_fkey,
    DROP CONSTRAINT IF EXISTS posts_author_human_fkey,
    DROP COLUMN IF EXISTS historical_author,
    DROP COLUMN IF EXISTS author_agent_id,
    DROP COLUMN IF EXISTS author_human_id;

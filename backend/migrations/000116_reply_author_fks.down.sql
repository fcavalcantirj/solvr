-- Back to the 000115 authorship: author_type/author_id with no foreign key. The dropped
-- columns are derived from those (and from legacy_id), so no data is lost.
DROP TRIGGER IF EXISTS replies_resolve_author ON replies;
DROP FUNCTION IF EXISTS replies_resolve_author();
ALTER TABLE replies
    DROP CONSTRAINT IF EXISTS replies_author_matches_label,
    DROP CONSTRAINT IF EXISTS replies_exactly_one_author,
    DROP CONSTRAINT IF EXISTS replies_author_agent_fkey,
    DROP CONSTRAINT IF EXISTS replies_author_human_fkey,
    DROP COLUMN IF EXISTS historical_author,
    DROP COLUMN IF EXISTS author_agent_id,
    DROP COLUMN IF EXISTS author_human_id;

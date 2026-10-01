-- A post names its author through a foreign key (idx 68 step 3: explicit human or agent
-- author with an exactly-one-author constraint), the same way replies do since 000116.
-- posted_by_type/posted_by_id stay the attribution every reader uses; these columns tie it
-- to the account:
--
--   * author_human_id -> users, author_agent_id -> agents. A post links exactly one of them
--     (posts_exactly_one_author), and it is the account its posted_by_type/posted_by_id name
--     (posts_author_matches_label). An account that still authors posts, soft-deleted ones
--     included, cannot be hard-deleted (no ON DELETE action).
--   * historical_author: a post stored before this migration whose author has no account
--     keeps posted_by_type/posted_by_id as a label and links nothing. No account is invented
--     for it. It stays historical through edits that keep its label; a new post, or a post
--     relabelled to another author, must name an account. Posts have no later migration
--     writer (the knowledge cutover converts them in place), so no new historical post is
--     ever needed.
--
-- The trigger posts_resolve_author derives the three columns from posted_by_type and
-- posted_by_id on every insert and on any change of them, so writers keep writing only the
-- label. An author with no account is refused with the constraint name of the foreign key it
-- would break.
--
-- Rows already stored are resolved once below; the ones naming no account become historical.
-- Measured before writing this (local restores of the production dump, read-only): after the
-- purge (at 113) 444 agent and 188 human posts, before it 558 and 1667, all naming an existing
-- account; 0 would be historical. posts has no triggers, so these UPDATEs move no timestamp.
ALTER TABLE posts
    ADD COLUMN author_human_id uuid,
    ADD COLUMN author_agent_id varchar(50),
    ADD COLUMN historical_author boolean NOT NULL DEFAULT false;

UPDATE posts p SET author_agent_id = a.id
FROM agents a WHERE p.posted_by_type = 'agent' AND a.id = p.posted_by_id;
UPDATE posts p SET author_human_id = u.id
FROM users u WHERE p.posted_by_type = 'human' AND u.id::text = lower(p.posted_by_id);
UPDATE posts SET historical_author = true
WHERE author_human_id IS NULL AND author_agent_id IS NULL;

ALTER TABLE posts
    ADD CONSTRAINT posts_author_human_fkey FOREIGN KEY (author_human_id) REFERENCES users (id),
    ADD CONSTRAINT posts_author_agent_fkey FOREIGN KEY (author_agent_id) REFERENCES agents (id),
    ADD CONSTRAINT posts_exactly_one_author CHECK (
        CASE WHEN historical_author
             THEN num_nonnulls(author_human_id, author_agent_id) = 0 AND btrim(posted_by_id) <> ''
             ELSE num_nonnulls(author_human_id, author_agent_id) = 1 END),
    ADD CONSTRAINT posts_author_matches_label CHECK (
        (author_human_id IS NULL OR (posted_by_type = 'human' AND author_human_id::text = lower(posted_by_id)))
        AND (author_agent_id IS NULL OR (posted_by_type = 'agent' AND author_agent_id = posted_by_id)));

CREATE INDEX idx_posts_author_human ON posts (author_human_id) WHERE author_human_id IS NOT NULL;
CREATE INDEX idx_posts_author_agent ON posts (author_agent_id) WHERE author_agent_id IS NOT NULL;

CREATE OR REPLACE FUNCTION posts_resolve_author() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    keeps_label boolean := TG_OP = 'UPDATE' AND OLD.historical_author
        AND NEW.posted_by_type IS NOT DISTINCT FROM OLD.posted_by_type
        AND NEW.posted_by_id IS NOT DISTINCT FROM OLD.posted_by_id;
BEGIN
    NEW.author_human_id := NULL;
    NEW.author_agent_id := NULL;
    NEW.historical_author := false;
    IF NEW.posted_by_type = 'agent' AND EXISTS (SELECT 1 FROM agents WHERE id = NEW.posted_by_id) THEN
        NEW.author_agent_id := NEW.posted_by_id;
    ELSIF NEW.posted_by_type = 'human'
          AND NEW.posted_by_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
          AND EXISTS (SELECT 1 FROM users WHERE id = NEW.posted_by_id::uuid) THEN
        NEW.author_human_id := NEW.posted_by_id::uuid;
    ELSIF keeps_label THEN
        NEW.historical_author := true;
    ELSE
        RAISE EXCEPTION 'post author % "%" has no account', NEW.posted_by_type, NEW.posted_by_id
            USING ERRCODE = 'foreign_key_violation',
                  CONSTRAINT = CASE WHEN NEW.posted_by_type = 'agent' THEN 'posts_author_agent_fkey'
                                    ELSE 'posts_author_human_fkey' END;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER posts_resolve_author
    BEFORE INSERT OR UPDATE OF posted_by_type, posted_by_id, author_human_id, author_agent_id, historical_author
    ON posts FOR EACH ROW EXECUTE FUNCTION posts_resolve_author();

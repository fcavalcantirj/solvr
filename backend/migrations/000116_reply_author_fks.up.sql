-- A reply names its author through a foreign key (idx 68 step 3: explicit human or agent
-- author with an exactly-one-author constraint). author_type/author_id stay the attribution
-- every reader uses; these columns tie it to the account:
--
--   * author_human_id -> users, author_agent_id -> agents. A human or agent reply links
--     exactly one of them (replies_exactly_one_author), and it is the account its
--     author_type/author_id name (replies_author_matches_label). An account that still
--     authors replies cannot be hard-deleted (no ON DELETE action).
--   * system replies (the moderator's notes) name no account: author_id is their label.
--   * historical_author: a reply the contribution cutover migrated (legacy_id set) whose
--     author has no account keeps author_type/author_id as a label and links nothing. No
--     account is invented for it. A native reply (legacy_id NULL) must name an account.
--
-- The trigger replies_resolve_author derives the three columns from author_type, author_id
-- and legacy_id on every insert and on any change of them, so writers keep writing only
-- author_type/author_id. A non-UUID human author is refused with the same constraint name
-- the foreign key uses.
--
-- Rows already stored are resolved once below; the ones naming no account become historical.
-- Measured before writing this (local restores of the production dump at 113, after the
-- purge): 446 agent and 13 human replies all name an existing account, 559 system notes;
-- 0 would be historical. The legacy tables the cutover migrates have 0 unresolved authors too.
ALTER TABLE replies
    ADD COLUMN author_human_id uuid,
    ADD COLUMN author_agent_id varchar(50),
    ADD COLUMN historical_author boolean NOT NULL DEFAULT false;

UPDATE replies r SET author_agent_id = a.id
FROM agents a WHERE r.author_type = 'agent' AND a.id = r.author_id;
UPDATE replies r SET author_human_id = u.id
FROM users u WHERE r.author_type = 'human' AND u.id::text = lower(r.author_id);
UPDATE replies SET historical_author = true
WHERE author_type IN ('human', 'agent') AND author_human_id IS NULL AND author_agent_id IS NULL;

ALTER TABLE replies
    ADD CONSTRAINT replies_author_human_fkey FOREIGN KEY (author_human_id) REFERENCES users (id),
    ADD CONSTRAINT replies_author_agent_fkey FOREIGN KEY (author_agent_id) REFERENCES agents (id),
    ADD CONSTRAINT replies_exactly_one_author CHECK (
        CASE WHEN author_type = 'system' OR historical_author
             THEN num_nonnulls(author_human_id, author_agent_id) = 0 AND btrim(author_id) <> ''
             ELSE num_nonnulls(author_human_id, author_agent_id) = 1 END),
    ADD CONSTRAINT replies_author_matches_label CHECK (
        (author_human_id IS NULL OR (author_type = 'human' AND author_human_id::text = lower(author_id)))
        AND (author_agent_id IS NULL OR (author_type = 'agent' AND author_agent_id = author_id))
        AND NOT (historical_author AND author_type = 'system'));

CREATE INDEX idx_replies_author_human ON replies (author_human_id) WHERE author_human_id IS NOT NULL;
CREATE INDEX idx_replies_author_agent ON replies (author_agent_id) WHERE author_agent_id IS NOT NULL;

CREATE OR REPLACE FUNCTION replies_resolve_author() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    human_key boolean := NEW.author_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
BEGIN
    NEW.author_human_id := NULL;
    NEW.author_agent_id := NULL;
    NEW.historical_author := false;
    IF NEW.author_type = 'agent' THEN
        IF NEW.legacy_id IS NULL OR EXISTS (SELECT 1 FROM agents WHERE id = NEW.author_id) THEN
            NEW.author_agent_id := NEW.author_id;
        ELSE
            NEW.historical_author := true;
        END IF;
    ELSIF NEW.author_type = 'human' THEN
        IF human_key AND (NEW.legacy_id IS NULL OR EXISTS (SELECT 1 FROM users WHERE id = NEW.author_id::uuid)) THEN
            NEW.author_human_id := NEW.author_id::uuid;
        ELSIF NEW.legacy_id IS NOT NULL THEN
            NEW.historical_author := true;
        ELSE
            RAISE EXCEPTION 'reply author "%" is not a human account', NEW.author_id
                USING ERRCODE = 'foreign_key_violation', CONSTRAINT = 'replies_author_human_fkey';
        END IF;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER replies_resolve_author
    BEFORE INSERT OR UPDATE OF author_type, author_id, author_human_id, author_agent_id, historical_author, legacy_id
    ON replies FOR EACH ROW EXECUTE FUNCTION replies_resolve_author();

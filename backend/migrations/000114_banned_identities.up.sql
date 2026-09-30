-- Anti-abuse (2026-09-29 purge follow-up): a ban list that outlives the banned account.
--
-- A soft-deleted (tombstoned) account keeps its email, so it already blocks a signup with
-- that exact email. A ban row goes further: it survives a hard delete, matches emails
-- case-insensitively, and can name an OAuth identity or an agent id with no account behind
-- it. The API refuses a banned OR tombstoned identity at every sign-in and registration
-- entry point (db.BannedIdentityRepository.IsRefused).
--
-- users_refuse_tombstoned_email: on production this replaces the emergency trigger installed
-- by hand on 2026-09-29. On a fresh database (tests, new environments) 114 CREATES it. 114
-- down restores the original function body and leaves the function and the trigger in place
-- on both kinds of database: the trigger is the pre-ban-list emergency ban and is never
-- removed by a rollback. Consequence: once 114 has run, any INSERT into users whose email
-- matches (case-insensitively) a soft-deleted user's email, or a banned email, fails with
-- 'account suspended' (P0001).
--
-- Also here, so this release carries one migration:
--   * idx_replies_author_created: the content gate and the create limiter read an author's
--     replies on every create, and replies had no author index;
--   * flags accepts reason moderation_rejected / moderation_failed and target progress_note,
--     which asynchronous moderation writes (moderation_failed was already written by
--     handlers/posts_moderation.go and rejected by the old CHECK).

-- COLLATE "C" keeps this table's unique index independent of the OS collation library.
CREATE TABLE IF NOT EXISTS banned_identities (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT COLLATE "C" NOT NULL CHECK (kind IN ('email', 'oauth', 'agent_id')),
    -- OAuth provider (github, google) for kind 'oauth'; empty otherwise.
    provider    TEXT COLLATE "C" NOT NULL DEFAULT '',
    -- Lowercased email, the provider's user id, or the agent id.
    value       TEXT COLLATE "C" NOT NULL,
    reason      TEXT NOT NULL,
    -- The account whose ban produced this row, when there was one.
    source_type TEXT,
    source_id   TEXT,
    created_by  TEXT NOT NULL DEFAULT 'operator',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT banned_identities_unique UNIQUE (kind, provider, value),
    CONSTRAINT banned_identities_email_lower CHECK (kind <> 'email' OR value = lower(value)),
    CONSTRAINT banned_identities_provider_kind CHECK ((kind = 'oauth') = (provider <> ''))
);

-- Seed: the accounts tombstoned by the 2026-09-29 purge. srcjj777 (owner of frogtrader) is
-- deliberately not banned.
INSERT INTO banned_identities (kind, provider, value, reason, source_type, source_id, created_by)
SELECT 'email', '', lower(u.email), '2026-09-29 purge: bot and repeated content', 'human', u.id::text, 'migration 000114'
FROM users u
WHERE u.username IN ('xiezhen223600', 'gongli0929', 'shan_he', 'xu_wei') AND u.email IS NOT NULL AND u.email <> ''
ON CONFLICT DO NOTHING;

INSERT INTO banned_identities (kind, provider, value, reason, source_type, source_id, created_by)
SELECT DISTINCT 'oauth', ident.provider, ident.provider_id, '2026-09-29 purge: bot and repeated content', 'human', ident.user_id, 'migration 000114'
FROM (
    SELECT u.id::text AS user_id, am.auth_provider AS provider, am.auth_provider_id AS provider_id
    FROM auth_methods am JOIN users u ON u.id = am.user_id
    WHERE u.username IN ('xiezhen223600', 'gongli0929', 'shan_he', 'xu_wei')
    UNION
    SELECT u.id::text, u.auth_provider, u.auth_provider_id
    FROM users u
    WHERE u.username IN ('xiezhen223600', 'gongli0929', 'shan_he', 'xu_wei')
) ident
WHERE ident.provider IN ('github', 'google') AND ident.provider_id IS NOT NULL AND ident.provider_id <> ''
ON CONFLICT DO NOTHING;

INSERT INTO banned_identities (kind, provider, value, reason, source_type, source_id, created_by)
SELECT 'agent_id', '', a.id, '2026-09-29 purge: bot and repeated content', 'agent', a.id, 'migration 000114'
FROM agents a
WHERE a.id IN ('agent_NaoParis', 'agent_frogtrader', 'agent_openclaw_mack')
ON CONFLICT DO NOTHING;

DO $$
DECLARE e int; o int; a int;
BEGIN
    SELECT count(*) INTO e FROM banned_identities WHERE kind = 'email';
    SELECT count(*) INTO o FROM banned_identities WHERE kind = 'oauth';
    SELECT count(*) INTO a FROM banned_identities WHERE kind = 'agent_id';
    RAISE NOTICE '000114: banned % emails, % oauth identities, % agents', e, o, a;
END $$;

CREATE INDEX IF NOT EXISTS idx_replies_author_created ON replies (author_type, author_id, created_at);

ALTER TABLE flags DROP CONSTRAINT IF EXISTS flags_reason_check;
ALTER TABLE flags ADD CONSTRAINT flags_reason_check
    CHECK (reason IN ('spam', 'offensive', 'duplicate', 'incorrect', 'low_quality', 'other', 'moderation_rejected', 'moderation_failed'));
ALTER TABLE flags DROP CONSTRAINT IF EXISTS flags_target_type_check;
ALTER TABLE flags ADD CONSTRAINT flags_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'comment', 'reply', 'progress_note'));

CREATE OR REPLACE FUNCTION users_refuse_tombstoned_email() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.email IS NOT NULL AND (EXISTS (
        SELECT 1 FROM users u WHERE lower(u.email) = lower(NEW.email) AND u.deleted_at IS NOT NULL)
       OR EXISTS (
        SELECT 1 FROM banned_identities b WHERE b.kind = 'email' AND b.value = lower(NEW.email) COLLATE "C")) THEN
        RAISE EXCEPTION 'account suspended' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END $$;
DROP TRIGGER IF EXISTS users_refuse_tombstoned_email ON users;
CREATE TRIGGER users_refuse_tombstoned_email BEFORE INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION users_refuse_tombstoned_email();

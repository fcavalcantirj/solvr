-- Reverse 000114. Nothing is dropped silently: ban rows and the flags the old CHECKs reject
-- are ARCHIVED into rollback_archive first, and the counts are raised as NOTICEs.
--
-- users_refuse_tombstoned_email is NOT dropped. Its original body (the emergency trigger
-- installed on production on 2026-09-29, verbatim below) is restored and the trigger stays,
-- on production and on a fresh database alike: until the ban list is back, a tombstone is the
-- only ban, and this trigger is what keeps a tombstoned email from signing up again.

CREATE OR REPLACE FUNCTION users_refuse_tombstoned_email() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.email IS NOT NULL AND EXISTS (
        SELECT 1 FROM users u WHERE lower(u.email) = lower(NEW.email) AND u.deleted_at IS NOT NULL) THEN
        RAISE EXCEPTION 'account suspended' USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END $$;

CREATE TABLE IF NOT EXISTS rollback_archive (
    id           BIGSERIAL PRIMARY KEY,
    source_table TEXT        NOT NULL,
    reason       TEXT        NOT NULL,
    row_data     JSONB       NOT NULL,
    archived_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

DO $$
DECLARE f int; b int;
BEGIN
    SELECT count(*) INTO f FROM flags
    WHERE reason IN ('moderation_rejected', 'moderation_failed') OR target_type = 'progress_note';
    SELECT count(*) INTO b FROM banned_identities;
    RAISE NOTICE '000114 down: archived and removed % flags the old CHECKs reject, % ban rows', f, b;
END $$;

INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'flags', '000114 down: flag reason or target type the old schema rejects', to_jsonb(f)
FROM flags f WHERE f.reason IN ('moderation_rejected', 'moderation_failed') OR f.target_type = 'progress_note';
DELETE FROM flags WHERE reason IN ('moderation_rejected', 'moderation_failed') OR target_type = 'progress_note';

ALTER TABLE flags DROP CONSTRAINT IF EXISTS flags_reason_check;
ALTER TABLE flags ADD CONSTRAINT flags_reason_check CHECK (reason IN ('spam', 'offensive', 'duplicate', 'incorrect', 'low_quality', 'other'));
ALTER TABLE flags DROP CONSTRAINT IF EXISTS flags_target_type_check;
ALTER TABLE flags ADD CONSTRAINT flags_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'comment', 'reply'));

DROP INDEX IF EXISTS idx_replies_author_created;

INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'banned_identities', '000114 down: ban list', to_jsonb(b) FROM banned_identities b;
DROP TABLE IF EXISTS banned_identities;

-- Reverse the canonical Reply model (BART-585).
--
-- The pre-reply schema cannot hold a vote or report on a reply, nor a reply itself. Rows
-- written after the cutover are therefore ARCHIVED into rollback_archive before they are
-- removed, and the counts are raised as NOTICEs: nothing is dropped silently. The archive
-- table outlives this migration on purpose (the old code never reads it).

CREATE TABLE IF NOT EXISTS rollback_archive (
    id           BIGSERIAL PRIMARY KEY,
    source_table TEXT        NOT NULL,
    reason       TEXT        NOT NULL,
    row_data     JSONB       NOT NULL,
    archived_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'votes', '000089 down: vote targets a reply', to_jsonb(v) FROM votes v WHERE v.target_type = 'reply';
INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'reports', '000089 down: report targets a reply', to_jsonb(r) FROM reports r WHERE r.target_type = 'reply';
INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'replies', '000089 down: reply created after the cutover (no legacy row)', to_jsonb(r) - 'embedding'
FROM replies r WHERE r.legacy_type IS NULL;

DO $$
DECLARE v int; rp int; rl int;
BEGIN
    SELECT count(*) INTO v  FROM votes   WHERE target_type = 'reply';
    SELECT count(*) INTO rp FROM reports WHERE target_type = 'reply';
    SELECT count(*) INTO rl FROM replies WHERE legacy_type IS NULL;
    RAISE NOTICE '000089 down: archived and removed % reply votes, % reply reports, % native replies', v, rp, rl;
END $$;

DELETE FROM votes   WHERE target_type = 'reply';
DELETE FROM reports WHERE target_type = 'reply';

ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check
    CHECK (target_type IN ('post', 'answer', 'approach', 'response', 'comment'));

ALTER TABLE votes DROP CONSTRAINT IF EXISTS votes_target_type_check;
ALTER TABLE votes ADD CONSTRAINT votes_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'blog_post'));

DROP TABLE IF EXISTS replies;

-- Rows the old schema cannot hold are archived in rollback_archive before they are removed
-- (the same table 000089.down and 000088.down write to), never dropped silently.
CREATE TABLE IF NOT EXISTS rollback_archive (
    id           BIGSERIAL PRIMARY KEY,
    source_table TEXT        NOT NULL,
    reason       TEXT        NOT NULL,
    row_data     JSONB       NOT NULL,
    archived_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Notification links on /posts (rewritten by the cutover, or written after it) go back to the
-- legacy page of the post's type: the old frontend has no /posts route. An anchor on a reply
-- migrated from a legacy row becomes that row's legacy anchor; any other reply anchor is
-- dropped, since the reply is not kept. A canonical 'post' becomes an idea, as in 000088.down.
-- A link to a post that no longer exists keeps its /posts path: its type is unknowable and the
-- link was already dead (the cutover rehearsal found 4 such links to hard-deleted posts).
WITH parsed AS (
    SELECT n.id,
           substring(n.link FROM '^/posts/([0-9a-fA-F-]{36})')::uuid         AS post_id,
           substring(n.link FROM '^/posts/[0-9a-fA-F-]{36}(.*)$')            AS rest,
           substring(n.link FROM '^/posts/[0-9a-fA-F-]{36}#([0-9a-fA-F-]{36})$') AS anchor
    FROM notifications n
    WHERE n.link ~ '^/posts/[0-9a-fA-F-]{36}([/?#]|$)'
)
UPDATE notifications n
SET link = '/' || CASE p.type WHEN 'problem' THEN 'problems' WHEN 'question' THEN 'questions' ELSE 'ideas' END
           || '/' || p.id::text
           || CASE WHEN x.anchor IS NULL THEN x.rest
                   WHEN r.legacy_type IN ('approach', 'answer', 'response', 'comment')
                        THEN '#' || r.legacy_type || '-' || r.legacy_id::text
                   ELSE '' END
FROM parsed x
JOIN posts p ON p.id = x.post_id
LEFT JOIN replies r ON r.id = x.anchor::uuid
WHERE n.id = x.id;

-- Flags retargeted to replies go back to the legacy row the reply was migrated from; a flag
-- on a reply with no legacy origin (created after cutover) is archived, then removed.
UPDATE flags SET target_type = r.legacy_type, target_id = r.legacy_id
FROM replies r
WHERE flags.target_type = 'reply' AND r.id = flags.target_id
  AND r.legacy_type IN ('approach', 'answer', 'response', 'comment');
INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'flags', '000109 down: flag targets a reply with no legacy row', to_jsonb(f)
FROM flags f WHERE f.target_type = 'reply';
DELETE FROM flags WHERE target_type = 'reply';
ALTER TABLE flags DROP CONSTRAINT flags_target_type_check;
ALTER TABLE flags ADD CONSTRAINT flags_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'comment'));

-- Replies migrated from progress notes are derived copies: the notes stay in progress_notes
-- until the legacy tables are dropped, so removing the copies loses nothing. Their deletion
-- cascades to replies written under them after cutover, so those are archived first.
WITH RECURSIVE under_note AS (
    SELECT r.id FROM replies r
    JOIN replies n ON n.id = r.parent_reply_id AND n.legacy_type = 'progress_note'
    WHERE r.legacy_type IS DISTINCT FROM 'progress_note'
    UNION ALL
    SELECT r.id FROM replies r JOIN under_note u ON r.parent_reply_id = u.id
)
INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'replies', '000109 down: reply under a progress note copy', to_jsonb(r) - 'embedding'
FROM replies r JOIN under_note u ON u.id = r.id;
DELETE FROM replies WHERE legacy_type = 'progress_note';
ALTER TABLE replies DROP CONSTRAINT replies_legacy_type_check;
ALTER TABLE replies ADD CONSTRAINT replies_legacy_type_check
    CHECK (legacy_type IN ('approach', 'answer', 'response', 'comment'));

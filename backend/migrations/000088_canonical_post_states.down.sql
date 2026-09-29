-- Reverse BART-583 canonical post state columns.

-- The old schema has no untyped post. Posts created after cutover with the canonical type
-- 'post' are archived in rollback_archive (with their canonical states, before those
-- columns go) and relabeled 'idea', the least structured legacy type, so no content is lost.
CREATE TABLE IF NOT EXISTS rollback_archive (
    id           BIGSERIAL PRIMARY KEY,
    source_table TEXT        NOT NULL,
    reason       TEXT        NOT NULL,
    row_data     JSONB       NOT NULL,
    archived_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO rollback_archive (source_table, reason, row_data)
SELECT 'posts', '000088 down: canonical post relabeled idea', to_jsonb(p) - 'embedding'
FROM posts p WHERE p.type = 'post';
UPDATE posts SET type = 'idea' WHERE type = 'post';

DROP INDEX IF EXISTS idx_posts_source_room;
DROP INDEX IF EXISTS idx_posts_public_eligible;

ALTER TABLE posts DROP COLUMN IF EXISTS source_room_id;

ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_moderation_state_check;
ALTER TABLE posts DROP COLUMN IF EXISTS moderation_state;

ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_publication_state_check;
ALTER TABLE posts DROP COLUMN IF EXISTS publication_state;

-- Restore the typed-only constraint (no 'post' rows remain after the relabel above).
ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts ADD CONSTRAINT posts_type_check
    CHECK (type IN ('problem', 'question', 'idea'));

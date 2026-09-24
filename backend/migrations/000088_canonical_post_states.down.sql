-- Reverse BART-583 canonical post state columns.
DROP INDEX IF EXISTS idx_posts_source_room;
DROP INDEX IF EXISTS idx_posts_public_eligible;

ALTER TABLE posts DROP COLUMN IF EXISTS source_room_id;

ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_moderation_state_check;
ALTER TABLE posts DROP COLUMN IF EXISTS moderation_state;

ALTER TABLE posts DROP CONSTRAINT IF EXISTS posts_publication_state_check;
ALTER TABLE posts DROP COLUMN IF EXISTS publication_state;

-- Restore the typed-only constraint (fails if canonical 'post' rows exist).
ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts ADD CONSTRAINT posts_type_check
    CHECK (type IN ('problem', 'question', 'idea'));

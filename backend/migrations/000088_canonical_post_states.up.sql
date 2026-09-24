-- Canonical Post contract (BART-583): separate the publication lifecycle from the
-- moderation decision, add optional room provenance, and allow an untyped canonical
-- post so POST /v1/posts no longer requires a problem/question/idea choice.

-- Allow the canonical untyped post type alongside the legacy typed values.
ALTER TABLE posts DROP CONSTRAINT posts_type_check;
ALTER TABLE posts ADD CONSTRAINT posts_type_check
    CHECK (type IN ('problem', 'question', 'idea', 'post'));

-- Publication lifecycle, independent of moderation.
ALTER TABLE posts ADD COLUMN IF NOT EXISTS publication_state VARCHAR(20) NOT NULL DEFAULT 'draft';
ALTER TABLE posts ADD CONSTRAINT posts_publication_state_check
    CHECK (publication_state IN ('draft', 'published', 'archived'));

-- Moderation decision, independent of the publication lifecycle. An author can move
-- publication_state but never moderation_state, so editing cannot bypass moderation.
ALTER TABLE posts ADD COLUMN IF NOT EXISTS moderation_state VARCHAR(20) NOT NULL DEFAULT 'pending';
ALTER TABLE posts ADD CONSTRAINT posts_moderation_state_check
    CHECK (moderation_state IN ('pending', 'approved', 'rejected'));

-- Optional provenance: the room a post was saved from (used later by "Save as post").
ALTER TABLE posts ADD COLUMN IF NOT EXISTS source_room_id UUID;

-- Backfill the new states from the legacy status so existing rows are consistent.
-- Mapping mirrors the migration contract: draft/pending_review -> draft+pending,
-- rejected -> draft+rejected, closed -> archived+approved, every other live status
-- (open/in_progress/solved/answered/active/dormant/evolved/stale) -> published+approved.
UPDATE posts SET publication_state = 'draft',     moderation_state = 'pending'
    WHERE status IN ('draft', 'pending_review');
UPDATE posts SET publication_state = 'draft',     moderation_state = 'rejected'
    WHERE status = 'rejected';
UPDATE posts SET publication_state = 'archived',  moderation_state = 'approved'
    WHERE status = 'closed';
UPDATE posts SET publication_state = 'published', moderation_state = 'approved'
    WHERE status IN ('open', 'in_progress', 'solved', 'answered', 'active', 'dormant', 'evolved', 'stale');

-- Public eligibility (published + approved + public, not deleted) is the common anon read.
CREATE INDEX IF NOT EXISTS idx_posts_public_eligible
    ON posts (created_at DESC)
    WHERE publication_state = 'published' AND moderation_state = 'approved'
      AND visibility = 'public' AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_posts_source_room
    ON posts (source_room_id) WHERE source_room_id IS NOT NULL;

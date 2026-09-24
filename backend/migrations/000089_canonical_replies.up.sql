-- Canonical Reply model (BART-585): one unified reply table replacing the
-- legacy approach/answer/response/comment split for all NEW contributions.
-- Legacy tables remain during the documented transition; migrated rows carry
-- their origin in legacy_type/legacy_id/provenance.

CREATE TABLE replies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- The post this reply contributes to (required).
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,

    -- Optional threading: a reply may be a child of another reply in the same post.
    parent_reply_id UUID REFERENCES replies(id) ON DELETE CASCADE,

    -- Author (polymorphic: human or agent).
    -- 'system' is included because migration 000054 widened comments.author_type
    -- to it for moderation-generated comments: 2678 of production's 2716 comments
    -- are system-authored, and migrating them verbatim is the whole point of the
    -- canonical model. Omitting it aborted MigrateContributions on the first one.
    author_type VARCHAR(10) NOT NULL CHECK (author_type IN ('human', 'agent', 'system')),
    author_id VARCHAR(255) NOT NULL,

    -- Free-form Markdown body: code, a failed attempt, a review, or discussion.
    -- No type-specific forms and no mandatory status workflow.
    body TEXT NOT NULL,

    -- Score is derived from confirmed votes (same pattern as posts).
    upvotes INTEGER NOT NULL DEFAULT 0,
    downvotes INTEGER NOT NULL DEFAULT 0,

    -- Migration provenance: which legacy table and row this reply was converted
    -- from, plus any preserved specialized fields as labeled JSON. NULL for
    -- natively created canonical replies.
    legacy_type VARCHAR(20) CHECK (legacy_type IN ('approach', 'answer', 'response', 'comment')),
    legacy_id UUID,
    provenance JSONB,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- List replies for a post oldest-to-newest, excluding soft-deleted rows.
CREATE INDEX idx_replies_post ON replies(post_id, created_at) WHERE deleted_at IS NULL;

-- Resolve children of a reply.
CREATE INDEX idx_replies_parent ON replies(parent_reply_id) WHERE parent_reply_id IS NOT NULL AND deleted_at IS NULL;

-- Idempotent migration: one canonical reply per legacy row so the contribution
-- migration can resume safely without duplicating replies.
CREATE UNIQUE INDEX idx_replies_legacy ON replies(legacy_type, legacy_id) WHERE legacy_id IS NOT NULL;

-- Votes and reports target the canonical Reply identity (target_type = 'reply').
-- Preserve every existing target type (votes already carries 'blog_post').
ALTER TABLE votes DROP CONSTRAINT IF EXISTS votes_target_type_check;
ALTER TABLE votes ADD CONSTRAINT votes_target_type_check
    CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'blog_post', 'reply'));

ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_target_type_check;
ALTER TABLE reports ADD CONSTRAINT reports_target_type_check
    CHECK (target_type IN ('post', 'answer', 'approach', 'response', 'comment', 'reply'));

COMMENT ON TABLE replies IS 'Canonical unified reply model (BART-585): one contribution type for all new knowledge; migrated legacy content carries provenance';

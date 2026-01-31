-- Comments table for approaches, answers, and responses
-- SPEC.md Part 2.6: Comments
-- SPEC.md Part 6: Database Schema

CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Target (polymorphic: approach, answer, or response)
    target_type VARCHAR(20) NOT NULL,
    target_id UUID NOT NULL,

    -- Author (polymorphic: human or agent)
    author_type VARCHAR(10) NOT NULL,
    author_id VARCHAR(255) NOT NULL,

    -- Comment content (max 2,000 chars per SPEC.md Part 2.6)
    content TEXT NOT NULL,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,  -- Soft delete

    -- Constraints
    CONSTRAINT comments_target_type_check CHECK (target_type IN ('approach', 'answer', 'response')),
    CONSTRAINT comments_author_type_check CHECK (author_type IN ('human', 'agent')),
    CONSTRAINT comments_content_not_empty CHECK (LENGTH(TRIM(content)) > 0),
    CONSTRAINT comments_content_max_length CHECK (LENGTH(content) <= 2000)
);

-- Indexes for common queries
-- Query comments by target (approach, answer, or response)
CREATE INDEX idx_comments_target ON comments(target_type, target_id);

-- Query comments by author
CREATE INDEX idx_comments_author ON comments(author_type, author_id);

-- Sort by created_at
CREATE INDEX idx_comments_created_at ON comments(created_at DESC);

-- Filter out soft-deleted comments
CREATE INDEX idx_comments_not_deleted ON comments(id) WHERE deleted_at IS NULL;

COMMENT ON TABLE comments IS 'Lightweight reactions on approaches, answers, or responses';
COMMENT ON COLUMN comments.target_type IS 'Type: approach, answer, response';
COMMENT ON COLUMN comments.content IS 'Markdown content, max 2000 chars';

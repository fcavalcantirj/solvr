-- Answers table
-- Answers to questions posted by humans or AI agents
-- See SPEC.md Part 2.4 and Part 6 for full schema documentation

CREATE TABLE answers (
    -- Core identity
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    question_id UUID NOT NULL REFERENCES posts(id),

    -- Author (polymorphic: human or agent)
    author_type VARCHAR(10) NOT NULL,
    author_id VARCHAR(255) NOT NULL,

    -- Content
    content TEXT NOT NULL,

    -- Status
    is_accepted BOOLEAN DEFAULT FALSE,

    -- Voting
    upvotes INT DEFAULT 0,
    downvotes INT DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Constraints
    CONSTRAINT answers_author_type_check CHECK (author_type IN ('human', 'agent')),
    CONSTRAINT answers_content_not_empty CHECK (length(trim(content)) > 0)
);

-- Lookup indexes
CREATE INDEX idx_answers_question ON answers(question_id);
CREATE INDEX idx_answers_author ON answers(author_type, author_id);
CREATE INDEX idx_answers_accepted ON answers(question_id) WHERE is_accepted = TRUE;
CREATE INDEX idx_answers_created ON answers(created_at DESC);

-- Soft delete filter (common query pattern)
CREATE INDEX idx_answers_not_deleted ON answers(id) WHERE deleted_at IS NULL;

-- Voting index for sorting
CREATE INDEX idx_answers_votes ON answers(question_id, (upvotes - downvotes) DESC);

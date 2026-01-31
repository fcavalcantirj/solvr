-- Responses table for ideas
-- SPEC.md Part 2.5: Responses (for Ideas)
-- SPEC.md Part 6: Database Schema

CREATE TABLE responses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Relationship to idea post
    idea_id UUID NOT NULL REFERENCES posts(id),

    -- Author (polymorphic: human or agent)
    author_type VARCHAR(10) NOT NULL,
    author_id VARCHAR(255) NOT NULL,

    -- Response content
    content TEXT NOT NULL,
    response_type VARCHAR(20) NOT NULL,

    -- Voting
    upvotes INT DEFAULT 0,
    downvotes INT DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Constraints
    CONSTRAINT responses_author_type_check CHECK (author_type IN ('human', 'agent')),
    CONSTRAINT responses_response_type_check CHECK (response_type IN ('build', 'critique', 'expand', 'question', 'support')),
    CONSTRAINT responses_content_not_empty CHECK (LENGTH(TRIM(content)) > 0)
);

-- Indexes for common queries
-- Query responses by idea
CREATE INDEX idx_responses_idea_id ON responses(idea_id);

-- Query responses by author
CREATE INDEX idx_responses_author ON responses(author_type, author_id);

-- Sort by created_at
CREATE INDEX idx_responses_created_at ON responses(created_at DESC);

-- Sort by votes (upvotes - downvotes descending)
CREATE INDEX idx_responses_votes ON responses((upvotes - downvotes) DESC);

-- Response type filtering
CREATE INDEX idx_responses_type ON responses(response_type);

COMMENT ON TABLE responses IS 'Responses to idea posts - discussions, critiques, expansions';
COMMENT ON COLUMN responses.response_type IS 'Type: build, critique, expand, question, support';

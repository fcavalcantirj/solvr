-- Posts table (polymorphic: problem, question, idea)
-- See SPEC.md Part 6 for full schema documentation

CREATE TABLE posts (
    -- Core identity
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(20) NOT NULL,

    -- Content
    title VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    tags TEXT[],

    -- Author (polymorphic: human or agent)
    posted_by_type VARCHAR(10) NOT NULL,
    posted_by_id VARCHAR(255) NOT NULL,

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'draft',

    -- Votes
    upvotes INT DEFAULT 0,
    downvotes INT DEFAULT 0,

    -- Problem-specific fields
    success_criteria TEXT[],
    weight INT,

    -- Question-specific fields
    accepted_answer_id UUID,

    -- Idea-specific fields
    evolved_into UUID[],

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Constraints
    CONSTRAINT posts_type_check CHECK (type IN ('problem', 'question', 'idea')),
    CONSTRAINT posts_posted_by_type_check CHECK (posted_by_type IN ('human', 'agent')),
    CONSTRAINT posts_status_check CHECK (status IN ('draft', 'open', 'in_progress', 'solved', 'answered', 'active', 'dormant', 'evolved', 'closed', 'stale')),
    CONSTRAINT posts_weight_check CHECK (weight IS NULL OR (weight >= 1 AND weight <= 5))
);

-- Full-text search index
CREATE INDEX idx_posts_search ON posts
    USING GIN(to_tsvector('english', title || ' ' || description));

-- Lookup indexes
CREATE INDEX idx_posts_type ON posts(type);
CREATE INDEX idx_posts_status ON posts(status);
CREATE INDEX idx_posts_tags ON posts USING GIN(tags);
CREATE INDEX idx_posts_created ON posts(created_at DESC);

-- Author lookup (for activity feeds)
CREATE INDEX idx_posts_author ON posts(posted_by_type, posted_by_id);

-- Soft delete filter (common query pattern)
CREATE INDEX idx_posts_not_deleted ON posts(id) WHERE deleted_at IS NULL;

-- Approaches table
-- A declared strategy for tackling a problem
-- See SPEC.md Part 2.3 and Part 6 for full schema documentation

CREATE TABLE approaches (
    -- Core identity
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    problem_id UUID NOT NULL REFERENCES posts(id),

    -- Author (polymorphic: human or agent)
    author_type VARCHAR(10) NOT NULL,
    author_id VARCHAR(255) NOT NULL,

    -- Approach details
    angle VARCHAR(500) NOT NULL,
    method VARCHAR(500),
    assumptions TEXT[],
    differs_from UUID[],

    -- Status
    status VARCHAR(20) NOT NULL DEFAULT 'starting',

    -- Results
    outcome TEXT,
    solution TEXT,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,

    -- Constraints
    CONSTRAINT approaches_author_type_check CHECK (author_type IN ('human', 'agent')),
    CONSTRAINT approaches_status_check CHECK (status IN ('starting', 'working', 'stuck', 'failed', 'succeeded'))
);

-- Progress notes table (linked to approaches)
CREATE TABLE progress_notes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    approach_id UUID NOT NULL REFERENCES approaches(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Lookup indexes
CREATE INDEX idx_approaches_problem ON approaches(problem_id);
CREATE INDEX idx_approaches_author ON approaches(author_type, author_id);
CREATE INDEX idx_approaches_status ON approaches(status);
CREATE INDEX idx_approaches_created ON approaches(created_at DESC);

-- Soft delete filter (common query pattern)
CREATE INDEX idx_approaches_not_deleted ON approaches(id) WHERE deleted_at IS NULL;

-- Progress notes lookup
CREATE INDEX idx_progress_notes_approach ON progress_notes(approach_id);
CREATE INDEX idx_progress_notes_created ON progress_notes(created_at DESC);

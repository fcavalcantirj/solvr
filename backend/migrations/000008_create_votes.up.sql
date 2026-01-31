-- Votes table for upvotes/downvotes on posts, answers, responses, approaches
-- SPEC.md Part 2.9: Votes
-- SPEC.md Part 6: Database Schema

CREATE TABLE votes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Target (polymorphic: post, answer, response, approach)
    target_type VARCHAR(20) NOT NULL,
    target_id UUID NOT NULL,

    -- Voter (polymorphic: human or agent)
    voter_type VARCHAR(10) NOT NULL,
    voter_id VARCHAR(255) NOT NULL,

    -- Vote direction (up or down)
    direction VARCHAR(4) NOT NULL,

    -- Confirmation status (Vote -> Confirm -> Locked)
    confirmed BOOLEAN DEFAULT FALSE,

    -- Timestamps
    created_at TIMESTAMPTZ DEFAULT NOW(),

    -- Constraints
    -- One vote per entity per target (SPEC.md Part 2.9)
    CONSTRAINT votes_unique_per_target UNIQUE (target_type, target_id, voter_type, voter_id),
    CONSTRAINT votes_target_type_check CHECK (target_type IN ('post', 'answer', 'response', 'approach')),
    CONSTRAINT votes_voter_type_check CHECK (voter_type IN ('human', 'agent')),
    CONSTRAINT votes_direction_check CHECK (direction IN ('up', 'down'))
);

-- Indexes for common queries
-- Query votes by target
CREATE INDEX idx_votes_target ON votes(target_type, target_id);

-- Query votes by voter (for checking if user already voted)
CREATE INDEX idx_votes_voter ON votes(voter_type, voter_id);

-- Query confirmed votes only (for tallying)
CREATE INDEX idx_votes_confirmed ON votes(target_type, target_id) WHERE confirmed = TRUE;

-- Sort by created_at
CREATE INDEX idx_votes_created_at ON votes(created_at DESC);

COMMENT ON TABLE votes IS 'Upvotes and downvotes on posts, answers, responses, and approaches';
COMMENT ON COLUMN votes.target_type IS 'Type: post, answer, response, approach';
COMMENT ON COLUMN votes.direction IS 'Vote direction: up or down';
COMMENT ON COLUMN votes.confirmed IS 'Once confirmed, vote is locked and cannot be changed';

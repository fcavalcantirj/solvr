-- Flags table for content moderation
-- SPEC.md Part 8.4: Content Moderation
-- Community flags: Any user can flag content, 3+ flags = hidden pending review

CREATE TABLE flags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Target (polymorphic: post, answer, response, approach, comment)
    target_type VARCHAR(50) NOT NULL,
    target_id UUID NOT NULL,

    -- Reporter (polymorphic: human, agent, or system for auto-detection)
    reporter_type VARCHAR(10) NOT NULL,
    reporter_id VARCHAR(255) NOT NULL,

    -- Flag reason (spam, offensive, duplicate, incorrect, other)
    reason VARCHAR(50) NOT NULL,

    -- Details provided by reporter (optional)
    details TEXT,

    -- Status: pending, reviewed, dismissed, actioned
    status VARCHAR(20) NOT NULL DEFAULT 'pending',

    -- Admin who reviewed (nullable until reviewed)
    reviewed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT flags_target_type_check CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'comment')),
    CONSTRAINT flags_reporter_type_check CHECK (reporter_type IN ('human', 'agent', 'system')),
    CONSTRAINT flags_reason_check CHECK (reason IN ('spam', 'offensive', 'duplicate', 'incorrect', 'low_quality', 'other')),
    CONSTRAINT flags_status_check CHECK (status IN ('pending', 'reviewed', 'dismissed', 'actioned')),
    -- Prevent duplicate flags from same reporter on same target
    CONSTRAINT flags_unique_per_reporter UNIQUE (target_type, target_id, reporter_type, reporter_id)
);

-- Indexes for common queries

-- Query flags by target (to check flag count on content)
CREATE INDEX idx_flags_target ON flags(target_type, target_id);

-- Query flags by status (for admin moderation queue)
CREATE INDEX idx_flags_status ON flags(status) WHERE status = 'pending';

-- Query pending flags sorted by creation date
CREATE INDEX idx_flags_pending_created ON flags(created_at DESC) WHERE status = 'pending';

-- Query flags by reporter (to track flagging patterns, prevent abuse)
CREATE INDEX idx_flags_reporter ON flags(reporter_type, reporter_id);

-- Query flags by reviewer (audit trail)
CREATE INDEX idx_flags_reviewed_by ON flags(reviewed_by) WHERE reviewed_by IS NOT NULL;

-- Count pending flags per target (for 3+ flags = hidden logic)
CREATE INDEX idx_flags_target_pending ON flags(target_type, target_id) WHERE status = 'pending';

-- Table and column comments
COMMENT ON TABLE flags IS 'Content flags for moderation - community and automated';
COMMENT ON COLUMN flags.target_type IS 'Type: post, answer, response, approach, comment';
COMMENT ON COLUMN flags.reporter_type IS 'Who flagged: human, agent, or system (auto-detection)';
COMMENT ON COLUMN flags.reason IS 'Flag reason: spam, offensive, duplicate, incorrect, low_quality, other';
COMMENT ON COLUMN flags.status IS 'Status: pending (new), reviewed (seen), dismissed (no action), actioned (action taken)';
COMMENT ON COLUMN flags.reviewed_by IS 'Admin who reviewed the flag (nullable until reviewed)';

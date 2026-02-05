-- Reports table for flagging inappropriate content
CREATE TABLE IF NOT EXISTS reports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    target_type VARCHAR(20) NOT NULL CHECK (target_type IN ('post', 'answer', 'approach', 'response', 'comment')),
    target_id UUID NOT NULL,
    reporter_type VARCHAR(10) NOT NULL CHECK (reporter_type IN ('human', 'agent')),
    reporter_id VARCHAR(255) NOT NULL,
    reason VARCHAR(50) NOT NULL CHECK (reason IN ('spam', 'offensive', 'off_topic', 'misleading', 'other')),
    details TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewed', 'actioned', 'dismissed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by VARCHAR(255),
    UNIQUE(target_type, target_id, reporter_type, reporter_id)
);

-- Index for querying reports by status
CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status);

-- Index for querying reports by target
CREATE INDEX IF NOT EXISTS idx_reports_target ON reports(target_type, target_id);

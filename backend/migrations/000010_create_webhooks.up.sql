-- Webhooks table for AI agent real-time notifications
-- Per SPEC.md Part 12.3

CREATE TABLE webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Agent relationship (required)
    agent_id VARCHAR(50) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Webhook configuration
    url VARCHAR(2048) NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{}',
    secret_hash VARCHAR(255) NOT NULL,

    -- Status: active, paused, failing, disabled
    status VARCHAR(20) NOT NULL DEFAULT 'active',

    -- Failure tracking
    consecutive_failures INT NOT NULL DEFAULT 0,
    last_failure_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,

    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT webhooks_status_check CHECK (status IN ('active', 'paused', 'failing', 'disabled')),
    CONSTRAINT webhooks_url_https_check CHECK (url LIKE 'https://%'),
    CONSTRAINT webhooks_url_not_empty CHECK (LENGTH(TRIM(url)) > 0),
    CONSTRAINT webhooks_events_not_empty CHECK (array_length(events, 1) > 0)
);

-- Comments for documentation
COMMENT ON TABLE webhooks IS 'Webhooks for AI agent real-time notifications';
COMMENT ON COLUMN webhooks.events IS 'Array of event types: answer.created, comment.created, approach.stuck, problem.solved, mention';
COMMENT ON COLUMN webhooks.status IS 'active=working, paused=manually paused, failing=recent failures, disabled=auto-disabled';
COMMENT ON COLUMN webhooks.secret_hash IS 'Bcrypt hash of webhook secret for HMAC-SHA256 signature verification';

-- Indexes
-- Look up webhooks by agent
CREATE INDEX idx_webhooks_agent_id ON webhooks(agent_id);

-- Find active webhooks quickly
CREATE INDEX idx_webhooks_agent_status ON webhooks(agent_id, status) WHERE status = 'active';

-- Find failing webhooks for monitoring
CREATE INDEX idx_webhooks_failing ON webhooks(status, consecutive_failures) WHERE status = 'failing';

-- Event subscription lookup (GIN for array contains)
CREATE INDEX idx_webhooks_events ON webhooks USING GIN(events);

-- Cleanup: find old disabled webhooks
CREATE INDEX idx_webhooks_disabled_updated ON webhooks(updated_at) WHERE status = 'disabled';

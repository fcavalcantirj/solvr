-- Rate limiting table
-- Stores rate limit counters per key (e.g., "agent:xyz:search", "user:abc:general")
-- Per SPEC.md Part 6

CREATE TABLE rate_limits (
    key VARCHAR(255) PRIMARY KEY,
    count INT NOT NULL DEFAULT 0,
    window_start TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for cleanup of old entries
CREATE INDEX idx_rate_limits_window_start ON rate_limits(window_start);

-- Comment on table
COMMENT ON TABLE rate_limits IS 'Rate limiting counters for API requests';
COMMENT ON COLUMN rate_limits.key IS 'Unique key identifying the rate limit bucket (e.g., agent:id:endpoint)';
COMMENT ON COLUMN rate_limits.count IS 'Number of requests in current window';
COMMENT ON COLUMN rate_limits.window_start IS 'Start time of current rate limit window';

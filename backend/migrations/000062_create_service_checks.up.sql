-- Service health check history for the status page.
-- Background job records checks every 5 minutes; 30-day chart + uptime stats derived from this table.

CREATE TABLE service_checks (
    id            BIGSERIAL    PRIMARY KEY,
    service_name  VARCHAR(50)  NOT NULL,   -- 'api', 'database', 'ipfs'
    status        VARCHAR(20)  NOT NULL,   -- 'operational', 'degraded', 'outage'
    response_time_ms INTEGER,              -- NULL when outage/timeout
    error_message TEXT,                    -- NULL when operational
    checked_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Latest check per service (status page live view)
CREATE INDEX idx_service_checks_service_time ON service_checks (service_name, checked_at DESC);

-- Daily aggregation queries (30-day uptime chart)
CREATE INDEX idx_service_checks_checked_at ON service_checks (checked_at);

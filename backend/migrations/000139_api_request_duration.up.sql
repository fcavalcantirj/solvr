-- Served duration for the operations report (spec.json idx 79).
--
-- duration_ms is how long the server took to answer one confirmed application
-- request: from the API boundary to the last byte the handler wrote. It is
-- server-side time and excludes the network, TLS and any proxy in front. Rows
-- recorded before this migration keep NULL: their duration was never measured
-- and is never estimated back. GET /admin/ops/slo computes p95 read and accepted
-- timeline-write latency from the rows that carry one.
ALTER TABLE api_request_events ADD COLUMN IF NOT EXISTS duration_ms INTEGER;

ALTER TABLE api_request_events DROP CONSTRAINT IF EXISTS api_request_events_duration_ms_check;
ALTER TABLE api_request_events ADD CONSTRAINT api_request_events_duration_ms_check
    CHECK (duration_ms IS NULL OR duration_ms >= 0);

COMMENT ON COLUMN api_request_events.duration_ms IS
    'Server-side milliseconds from the API boundary to the end of the handler. NULL when not measured (rows before 000139).';

-- A search that did not answer 2xx, kept so its input can be read (SPEC.md 22.8).
-- One row per GET /v1/search answered 400, 499 or 5xx after the q check: the input as the
-- server read it (valid UTF-8, NUL removed, at most 500 characters), the outcome and the
-- request id that the error body, the log line and api_request_events carry. Operator
-- diagnostics only; nothing prunes it by age (an owner decision, metric_retention.go).
CREATE TABLE IF NOT EXISTS search_failures (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT,
    query TEXT NOT NULL,
    status SMALLINT NOT NULL,
    error_code TEXT NOT NULL,
    sqlstate TEXT,
    searcher_type TEXT NOT NULL,
    duration_ms INTEGER,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT search_failures_query_length CHECK (char_length(query) <= 500),
    CONSTRAINT search_failures_status_check CHECK (status BETWEEN 400 AND 599),
    CONSTRAINT search_failures_searcher_type_check CHECK (searcher_type IN ('agent', 'human', 'anonymous')),
    CONSTRAINT search_failures_duration_check CHECK (duration_ms IS NULL OR duration_ms >= 0)
);

CREATE INDEX IF NOT EXISTS idx_search_failures_occurred_at ON search_failures (occurred_at DESC);

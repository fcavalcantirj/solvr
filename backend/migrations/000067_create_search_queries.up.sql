-- Track search queries for analytics: trending searches, content gaps, user vs agent patterns.
-- Async fire-and-forget INSERT per search request, no latency impact.

CREATE TABLE search_queries (
    id               BIGSERIAL    PRIMARY KEY,
    query            TEXT         NOT NULL CHECK (length(query) <= 500),
    query_normalized TEXT         NOT NULL CHECK (length(query_normalized) <= 500),
    type_filter      VARCHAR(20),
    results_count    INTEGER      NOT NULL,
    search_method    VARCHAR(20)  NOT NULL,
    duration_ms      INTEGER      NOT NULL,
    searcher_type    VARCHAR(10)  NOT NULL DEFAULT 'anonymous',
    searcher_id      VARCHAR(255),
    ip_address       INET,
    user_agent       VARCHAR(500),
    page             INTEGER      NOT NULL DEFAULT 1,
    searched_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Trending queries (GROUP BY normalized, ORDER BY count)
CREATE INDEX idx_search_queries_trending ON search_queries (query_normalized, searched_at DESC);

-- Content gaps (zero-result queries)
CREATE INDEX idx_search_queries_zero_results ON search_queries (searched_at DESC) WHERE results_count = 0;

-- Human vs agent breakdown
CREATE INDEX idx_search_queries_searcher ON search_queries (searcher_type, searched_at DESC);

-- Time-based cleanup
CREATE INDEX idx_search_queries_searched_at ON search_queries (searched_at);

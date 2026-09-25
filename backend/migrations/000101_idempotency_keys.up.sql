-- Durable Idempotency-Key store for create endpoints (idx 73 step 4).
--
-- One row per (actor, operation, key). A row is 'pending' while the first request
-- runs and 'completed' once its 2xx response is stored for replay. request_hash
-- (sha256 of method, path and body) lets a retry with a different payload be refused.
-- Rows are retained for at least 24 hours; after that, or after a pending row's
-- request is presumed dead, the key may be reserved again.
CREATE TABLE IF NOT EXISTS idempotency_keys (
    actor_type            VARCHAR(10)  NOT NULL,
    actor_id              TEXT         NOT NULL,
    operation             VARCHAR(64)  NOT NULL,
    idempotency_key       VARCHAR(255) NOT NULL,
    request_hash          TEXT         NOT NULL,
    status                VARCHAR(10)  NOT NULL DEFAULT 'pending'
                          CHECK (status IN ('pending', 'completed')),
    response_status       INTEGER,
    response_content_type TEXT,
    response_body         BYTEA,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at          TIMESTAMPTZ,
    PRIMARY KEY (actor_type, actor_id, operation, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_idempotency_keys_created_at ON idempotency_keys (created_at);

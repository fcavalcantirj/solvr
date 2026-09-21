-- Aggregate API usage, measured at the API boundary.
--
-- One row per CONFIRMED APPLICATION REQUEST: a request that matched a real
-- application route and was served by it. The row keeps only what an
-- aggregate call volume needs — the stable route TEMPLATE (never a slug, an
-- id or a query string), the canonical operation the template maps to, the
-- response class, the actor type and the instant.
--
-- What is deliberately NOT stored, because it must never be publishable:
-- request bodies, query strings, room slugs, account ids, IP addresses,
-- credentials and user agents. A row here can identify nobody.
--
-- request_id makes counting idempotent. A proxy retry or a route adapter
-- re-dispatching the same incoming request carries the same X-Request-ID, and
-- the unique index turns the second insert into a no-op: request volume counts
-- each INCOMING request once. Rows recorded before a request identifier was
-- available keep NULL, which the partial index leaves alone.
CREATE TABLE IF NOT EXISTS api_request_events (
    id BIGSERIAL PRIMARY KEY,
    request_id TEXT,
    route_template TEXT NOT NULL,
    method TEXT NOT NULL,
    operation TEXT NOT NULL,
    operation_family TEXT NOT NULL,
    operation_kind TEXT NOT NULL,
    actor_type TEXT NOT NULL,
    status_class SMALLINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT api_request_events_actor_type_check
        CHECK (actor_type IN ('agent', 'human', 'anonymous')),
    CONSTRAINT api_request_events_operation_kind_check
        CHECK (operation_kind IN ('poll', 'write', 'search')),
    CONSTRAINT api_request_events_operation_family_check
        CHECK (operation_family IN ('room', 'knowledge', 'identity', 'other')),
    CONSTRAINT api_request_events_status_class_check
        CHECK (status_class BETWEEN 1 AND 5)
);

COMMENT ON TABLE api_request_events IS
    'One confirmed application request at the API boundary. Aggregate call volume only: no bodies, no slugs, no ids, no IPs, no credentials.';
COMMENT ON COLUMN api_request_events.request_id IS
    'The incoming X-Request-ID. Unique, so proxy retries and route adapters cannot count one incoming request twice.';
COMMENT ON COLUMN api_request_events.operation IS
    'The canonical operation the route template maps to. Two adapters of the same operation (POST /v1/rooms/{slug}/messages and POST /r/{slug}/message) share one name.';
COMMENT ON COLUMN api_request_events.operation_kind IS
    'poll: a passive read. write: a create/send/update/delete. search: a knowledge search. Passive polling is reported apart from create/send/search.';

CREATE UNIQUE INDEX IF NOT EXISTS idx_api_request_events_request_id
    ON api_request_events (request_id)
    WHERE request_id IS NOT NULL;

-- The homepage reads windows ending now, newest first.
CREATE INDEX IF NOT EXISTS idx_api_request_events_occurred_at
    ON api_request_events (occurred_at DESC);

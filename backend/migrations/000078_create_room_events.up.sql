-- Typed, queryable room events (mission #4).
--
-- Structured coordination signals — CLAIM / BUILDING / PR / MERGED / RELEASE — that an
-- agent can query without scanning message history ("who holds APP-185 / what's building
-- now"). Distinct from messages (freeform chat) and from claims (locks): an event is an
-- append-only announcement with a machine-readable type, an optional issue reference, an
-- actor, and a JSON payload.
--
-- 'issue' is NOT NULL DEFAULT '' rather than nullable: an event may legitimately not
-- reference an issue (e.g. a RELEASE), and an empty string keeps every filter query a
-- plain equality with no NULL handling.
CREATE TABLE room_events (
    id         BIGSERIAL PRIMARY KEY,
    room_id    UUID  NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    event_type TEXT  NOT NULL CHECK (length(event_type) BETWEEN 1 AND 50),
    issue      TEXT  NOT NULL DEFAULT '' CHECK (length(issue) <= 200),
    actor      TEXT  NOT NULL CHECK (length(actor) BETWEEN 1 AND 200),
    payload    JSONB NOT NULL DEFAULT '{}' CHECK (length(payload::text) <= 16384),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Query by type ("what's building now") and by issue ("who holds APP-185"), newest first.
CREATE INDEX idx_room_events_room_type   ON room_events (room_id, event_type, id DESC);
CREATE INDEX idx_room_events_room_issue  ON room_events (room_id, issue, id DESC);
CREATE INDEX idx_room_events_room_recent ON room_events (room_id, id DESC);

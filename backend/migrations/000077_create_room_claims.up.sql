-- Distributed lock / lease primitive for rooms (mission #2).
--
-- A claim is a compare-and-set lock scoped to (room_id, claim_key). Agents use it to
-- coordinate exclusive work — e.g. "who is building issue APP-185" — so they stop
-- hand-rolling optimistic-claim-then-verify races. Acquisition is a single atomic
-- INSERT ... ON CONFLICT ... DO UPDATE WHERE expired statement (see RoomClaimRepository),
-- which PostgreSQL serializes: under concurrency exactly one caller wins a given key.
CREATE TABLE room_claims (
    room_id    UUID        NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    claim_key  TEXT        NOT NULL CHECK (length(claim_key) BETWEEN 1 AND 200),
    holder     TEXT        NOT NULL CHECK (length(holder) BETWEEN 1 AND 200),
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (room_id, claim_key)
);

-- Sweep/query live vs expired claims.
CREATE INDEX idx_room_claims_expires ON room_claims (expires_at);

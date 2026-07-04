-- Per-agent room credentials (mission #3).
--
-- Today a room has a single shared bearer token (rooms.token_hash): everyone uses the
-- same secret, a leak lets anyone in, and rotating it kicks every agent. This table
-- gives each agent its OWN room-scoped token (solvr_rt_...), issued after a handshake in
-- which the agent proves its identity with its Solvr agent API key. Message authorship
-- then becomes authoritative (the token identifies the agent), and one agent can be
-- revoked without rotating the shared token for everyone else.
--
-- expires_at is nullable on purpose: a per-agent token may be short-lived (set expiry)
-- or long-lived (NULL = never expires), per the operator's needs.
CREATE TABLE room_agent_tokens (
    room_id      UUID        NOT NULL REFERENCES rooms(id)  ON DELETE CASCADE,
    agent_id     VARCHAR(50) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL,
    expires_at   TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    PRIMARY KEY (room_id, agent_id)
);

-- BearerGuard resolves an incoming token by its hash.
CREATE UNIQUE INDEX idx_room_agent_tokens_hash ON room_agent_tokens (token_hash);

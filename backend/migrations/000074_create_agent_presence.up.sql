CREATE TABLE agent_presence (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id     UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    agent_name  VARCHAR(100) NOT NULL,
    card_json   JSONB NOT NULL
                    CHECK (length(card_json::text) <= 16384),
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ttl_seconds INT NOT NULL DEFAULT 900,
    UNIQUE (room_id, agent_name)
);

CREATE INDEX idx_agent_presence_room_id ON agent_presence (room_id);
CREATE INDEX idx_agent_presence_last_seen ON agent_presence (last_seen);

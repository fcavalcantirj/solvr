CREATE TABLE messages (
    id              BIGSERIAL PRIMARY KEY,
    room_id         UUID NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
    author_type     VARCHAR(10) NOT NULL DEFAULT 'agent'
                        CHECK (author_type IN ('human', 'agent', 'system')),
    author_id       VARCHAR(255),
    agent_name      VARCHAR(100) NOT NULL,
    content         TEXT NOT NULL
                        CHECK (length(content) <= 65536),
    content_type    VARCHAR(20) NOT NULL DEFAULT 'text'
                        CHECK (content_type IN ('text', 'markdown', 'json')),
    metadata        JSONB NOT NULL DEFAULT '{}',
    sequence_num    INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_messages_room_created ON messages (room_id, created_at);
CREATE INDEX idx_messages_room_active ON messages (room_id, created_at)
    WHERE deleted_at IS NULL;

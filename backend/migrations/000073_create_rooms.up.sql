CREATE TABLE rooms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT UNIQUE NOT NULL
                        CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$'),
    display_name    VARCHAR(200) NOT NULL,
    description     VARCHAR(1000),
    category        VARCHAR(50),
    tags            TEXT[] NOT NULL DEFAULT '{}'
                        CHECK (array_length(tags, 1) <= 10),
    is_private      BOOLEAN NOT NULL DEFAULT FALSE,
    owner_id        UUID REFERENCES users(id) ON DELETE SET NULL,
    token_hash      TEXT NOT NULL,
    message_count   INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    deleted_at      TIMESTAMPTZ
);

CREATE INDEX idx_rooms_owner_id ON rooms (owner_id);
CREATE INDEX idx_rooms_expires_at ON rooms (expires_at)
    WHERE expires_at IS NOT NULL;
CREATE INDEX idx_rooms_active ON rooms (last_active_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_rooms_deleted ON rooms (deleted_at)
    WHERE deleted_at IS NULL;

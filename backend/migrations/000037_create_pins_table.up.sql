-- Create pins table for IPFS Pinning Service API.
-- Follows the IPFS Pinning Service API spec for interoperability.
-- Each pin represents a request to pin content identified by a CID on IPFS.

CREATE TABLE IF NOT EXISTS pins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cid TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    name TEXT,
    origins TEXT[],
    meta JSONB,
    delegates TEXT[],
    owner_id TEXT NOT NULL,
    owner_type VARCHAR(10) NOT NULL,
    size_bytes BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    pinned_at TIMESTAMPTZ,

    -- Status must be one of the valid pin states
    CONSTRAINT pins_status_check CHECK (status IN ('queued', 'pinning', 'pinned', 'failed')),

    -- Owner type must be user or agent
    CONSTRAINT pins_owner_type_check CHECK (owner_type IN ('user', 'agent')),

    -- Same owner can't pin the same CID twice
    CONSTRAINT pins_cid_owner_unique UNIQUE (cid, owner_id)
);

-- Index on CID for fast lookup
CREATE INDEX IF NOT EXISTS idx_pins_cid ON pins(cid);

-- Index on owner for listing user's pins
CREATE INDEX IF NOT EXISTS idx_pins_owner ON pins(owner_id, owner_type);

-- Index on status for filtering
CREATE INDEX IF NOT EXISTS idx_pins_status ON pins(status);

-- Index on created_at for ordering
CREATE INDEX IF NOT EXISTS idx_pins_created_at ON pins(created_at DESC);

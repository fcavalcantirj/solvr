-- Agents table: AI agents registered on Solvr
-- See SPEC.md Part 6 for schema details

CREATE TABLE agents (
    id VARCHAR(50) PRIMARY KEY,
    display_name VARCHAR(50) NOT NULL,
    human_id UUID REFERENCES users(id),
    bio VARCHAR(500),
    specialties TEXT[],
    avatar_url TEXT,
    api_key_hash VARCHAR(255),
    moltbook_id VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index for looking up agents by owner (human)
CREATE INDEX idx_agents_human_id ON agents(human_id);

-- Index for Moltbook integration lookups
CREATE INDEX idx_agents_moltbook_id ON agents(moltbook_id) WHERE moltbook_id IS NOT NULL;

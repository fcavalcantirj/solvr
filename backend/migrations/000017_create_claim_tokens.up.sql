-- Claim tokens table: for agent-human linking flow
-- Agents generate claim tokens, humans confirm to link
-- See SPEC.md Part 12.3 and PRD AGENT-LINKING category

CREATE TABLE claim_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token VARCHAR(64) UNIQUE NOT NULL,
    agent_id VARCHAR(50) NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    used_by_human_id UUID REFERENCES users(id),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Index on token for fast lookup during claim flow
CREATE INDEX idx_claim_tokens_token ON claim_tokens(token);

-- Index on agent_id for listing agent's tokens
CREATE INDEX idx_claim_tokens_agent_id ON claim_tokens(agent_id);

-- Partial unique index: only one unused token per agent at a time
-- Expiry is checked in application code during validation
CREATE UNIQUE INDEX idx_claim_tokens_one_active_per_agent
    ON claim_tokens(agent_id)
    WHERE used_at IS NULL;

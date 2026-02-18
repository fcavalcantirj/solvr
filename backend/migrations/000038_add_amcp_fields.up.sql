-- Add AMCP (Autonomous Machine Communication Protocol) fields to agents table.
-- Per prd-v6-ipfs-expanded: AMCP agent detection — auto-enable pinning for agents with valid AMCP identity.

ALTER TABLE agents ADD COLUMN IF NOT EXISTS has_amcp_identity BOOLEAN DEFAULT false;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS amcp_aid VARCHAR(255);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS pinning_quota_bytes BIGINT DEFAULT 0;

-- Index for querying AMCP-enabled agents
CREATE INDEX IF NOT EXISTS idx_agents_has_amcp_identity ON agents(has_amcp_identity) WHERE has_amcp_identity = true;

-- Unique constraint on amcp_aid when provided (no two agents can have the same KERI AID)
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_amcp_aid_unique ON agents(amcp_aid) WHERE amcp_aid IS NOT NULL;

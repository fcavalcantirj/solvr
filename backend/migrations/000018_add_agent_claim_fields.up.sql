-- Add fields for agent-human linking per AGENT-LINKING requirement
-- These fields support the claim flow where humans verify ownership of agents

-- Add status field for agent account status (active, suspended)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active';

-- Add karma field for reputation points
-- Per AGENT-LINKING: +50 karma on human claim
ALTER TABLE agents ADD COLUMN IF NOT EXISTS karma INT NOT NULL DEFAULT 0;

-- Add human_claimed_at to track when a human claimed this agent
-- Per AGENT-LINKING requirement
ALTER TABLE agents ADD COLUMN IF NOT EXISTS human_claimed_at TIMESTAMPTZ;

-- Add has_human_backed_badge to indicate verified human ownership
-- Per AGENT-LINKING: granted on successful claim
ALTER TABLE agents ADD COLUMN IF NOT EXISTS has_human_backed_badge BOOLEAN NOT NULL DEFAULT FALSE;

-- Create a trigger function to prevent re-claiming an agent
-- Per AGENT-LINKING requirement: "CHECK constraint: human_id can only be set once"
-- Using a trigger instead of CHECK because CHECK can't reference old values
CREATE OR REPLACE FUNCTION prevent_agent_reclaim()
RETURNS TRIGGER AS $$
BEGIN
    -- If human_id was already set (not null) and we're trying to change it
    IF OLD.human_id IS NOT NULL AND NEW.human_id IS DISTINCT FROM OLD.human_id THEN
        RAISE EXCEPTION 'agent_already_claimed: Agent is already linked to a human and cannot be re-claimed'
            USING ERRCODE = 'P0001';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create the trigger on the agents table
DROP TRIGGER IF EXISTS trigger_prevent_agent_reclaim ON agents;
CREATE TRIGGER trigger_prevent_agent_reclaim
    BEFORE UPDATE ON agents
    FOR EACH ROW
    EXECUTE FUNCTION prevent_agent_reclaim();

-- Index on status for filtering (e.g., finding suspended agents)
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);

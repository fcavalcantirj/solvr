-- Add UNIQUE constraint on display_name for agents table
-- Per AGENT-ONBOARDING requirement: "Agents table: name UNIQUE constraint enforced"
-- Note: display_name serves as the agent's "name" in registration (POST /v1/agents/register)

-- First, add a UNIQUE constraint on display_name
-- Using CREATE UNIQUE INDEX instead of ALTER TABLE to handle potential duplicates gracefully
-- and to allow conditional index if needed in the future

CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_display_name_unique
ON agents (display_name);

-- Add a comment explaining the constraint
COMMENT ON INDEX idx_agents_display_name_unique IS
    'Ensures agent names are unique across the platform. Per AGENT-ONBOARDING requirement.';

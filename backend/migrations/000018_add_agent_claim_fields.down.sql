-- Rollback migration: remove agent claim fields

-- Drop the trigger first
DROP TRIGGER IF EXISTS trigger_prevent_agent_reclaim ON agents;

-- Drop the trigger function
DROP FUNCTION IF EXISTS prevent_agent_reclaim();

-- Drop the index
DROP INDEX IF EXISTS idx_agents_status;

-- Remove the columns
ALTER TABLE agents DROP COLUMN IF EXISTS has_human_backed_badge;
ALTER TABLE agents DROP COLUMN IF EXISTS human_claimed_at;
ALTER TABLE agents DROP COLUMN IF EXISTS karma;
ALTER TABLE agents DROP COLUMN IF EXISTS status;

-- Migration 000035: Add soft delete support for agents
-- Per PRD-v5 Task 22: Agent self-deletion feature
-- Adds deleted_at column and partial index for performance

-- Add deleted_at column to agents table
-- NULL = agent is active, NOT NULL = agent is soft-deleted
ALTER TABLE agents ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Create partial index to optimize queries filtering active agents
-- Most queries will use WHERE deleted_at IS NULL
-- Partial index is much smaller and faster than full index
CREATE INDEX IF NOT EXISTS idx_agents_deleted_at
ON agents(deleted_at)
WHERE deleted_at IS NULL;

-- Rollback for migration 000035: Remove agents soft delete support

-- Drop the partial index
DROP INDEX IF EXISTS idx_agents_deleted_at;

-- Remove deleted_at column from agents table
ALTER TABLE agents DROP COLUMN IF EXISTS deleted_at;

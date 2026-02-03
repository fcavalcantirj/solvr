-- Remove UNIQUE constraint on display_name for agents table
DROP INDEX IF EXISTS idx_agents_display_name_unique;

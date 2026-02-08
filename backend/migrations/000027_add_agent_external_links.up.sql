-- Add external_links field to agents table
-- Allows agents to link to external profiles (Moltbook, AgentArxiv, etc.)
ALTER TABLE agents ADD COLUMN external_links TEXT[];

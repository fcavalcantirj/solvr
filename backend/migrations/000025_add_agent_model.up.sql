-- Add model field to agents table
-- Per prd-v4: allows agents to specify their AI model (e.g., claude-opus-4, gpt-4-turbo)
ALTER TABLE agents ADD COLUMN model VARCHAR(100);

-- Add email field to agents table
-- Allows agents to have a contact email address
ALTER TABLE agents ADD COLUMN email VARCHAR(255);

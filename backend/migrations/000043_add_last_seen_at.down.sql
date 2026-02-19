-- Revert: remove last_seen_at from agents.
ALTER TABLE agents DROP COLUMN IF EXISTS last_seen_at;

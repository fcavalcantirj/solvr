-- Rename karma column to reputation for agents.
-- Unifies the two separate metrics (karma=bonuses, reputation=activity)
-- into a single "reputation" concept across the platform.
ALTER TABLE agents RENAME COLUMN karma TO reputation;

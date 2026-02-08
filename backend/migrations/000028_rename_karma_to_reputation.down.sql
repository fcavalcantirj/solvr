-- Revert: rename reputation back to karma
ALTER TABLE agents RENAME COLUMN reputation TO karma;

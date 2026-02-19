-- Revert agent default pinning quota back to 0.
ALTER TABLE agents ALTER COLUMN pinning_quota_bytes SET DEFAULT 0;

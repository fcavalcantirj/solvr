-- Change agent default pinning quota from 0 to 1GB (1073741824 bytes).
-- All registered agents get 1GB free IPFS pinning.
ALTER TABLE agents ALTER COLUMN pinning_quota_bytes SET DEFAULT 1073741824;

-- Update existing agents that still have 0 quota to get the new default.
UPDATE agents SET pinning_quota_bytes = 1073741824 WHERE pinning_quota_bytes = 0;

-- Reverse AMCP fields from agents table.

DROP INDEX IF EXISTS idx_agents_amcp_aid_unique;
DROP INDEX IF EXISTS idx_agents_has_amcp_identity;

ALTER TABLE agents DROP COLUMN IF EXISTS pinning_quota_bytes;
ALTER TABLE agents DROP COLUMN IF EXISTS amcp_aid;
ALTER TABLE agents DROP COLUMN IF EXISTS has_amcp_identity;

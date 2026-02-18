-- Add storage quota tracking columns to users and agents tables.
-- Per prd-v6-ipfs-expanded.json Phase 2: "Add storage quota tracking and enforcement"
--
-- storage_used_bytes: tracks total pinned content size for the account
-- storage_quota_bytes: maximum allowed storage (tier-based, default 100MB for users)
--
-- Agents already have pinning_quota_bytes from migration 000038 (AMCP fields).
-- We add storage_used_bytes to agents, and both columns to users.

-- Users: add storage tracking columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS storage_used_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS storage_quota_bytes BIGINT NOT NULL DEFAULT 104857600; -- 100MB default

-- Agents: add storage_used_bytes (pinning_quota_bytes already exists from migration 000038)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS storage_used_bytes BIGINT NOT NULL DEFAULT 0;

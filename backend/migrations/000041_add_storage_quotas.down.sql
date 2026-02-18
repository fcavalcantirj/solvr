-- Reverse storage quota tracking columns.

ALTER TABLE users DROP COLUMN IF EXISTS storage_used_bytes;
ALTER TABLE users DROP COLUMN IF EXISTS storage_quota_bytes;
ALTER TABLE agents DROP COLUMN IF EXISTS storage_used_bytes;

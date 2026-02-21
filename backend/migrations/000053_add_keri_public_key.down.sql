-- Reverse: remove KERI public key column and its unique index.

DROP INDEX IF EXISTS idx_agents_keri_public_key;
ALTER TABLE agents DROP COLUMN IF EXISTS keri_public_key;

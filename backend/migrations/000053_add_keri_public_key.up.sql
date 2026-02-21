-- Add KERI public key column to agents table for KERI identity management.
-- Part of AMCP identity verification: agents can store their KERI public key
-- alongside their AMCP AID for cryptographic identity proof.

ALTER TABLE agents ADD COLUMN IF NOT EXISTS keri_public_key TEXT;

-- Unique partial index: ensures no two agents share the same KERI public key.
-- NULL values are excluded (agents without KERI identity are unaffected).
CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_keri_public_key
    ON agents(keri_public_key)
    WHERE keri_public_key IS NOT NULL;

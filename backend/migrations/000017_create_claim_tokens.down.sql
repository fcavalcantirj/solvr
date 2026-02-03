-- Rollback claim_tokens table

DROP INDEX IF EXISTS idx_claim_tokens_one_active_per_agent;
DROP INDEX IF EXISTS idx_claim_tokens_agent_id;
DROP INDEX IF EXISTS idx_claim_tokens_token;
DROP TABLE IF EXISTS claim_tokens;

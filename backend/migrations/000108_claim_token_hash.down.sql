-- The clear tokens were not kept, so they cannot come back: every existing row gets its hash
-- as its token, which no link can present. Live claim links stop working; agents ask again.
ALTER TABLE claim_tokens ADD COLUMN token VARCHAR(64);
UPDATE claim_tokens SET token = token_hash;
ALTER TABLE claim_tokens ALTER COLUMN token SET NOT NULL;
ALTER TABLE claim_tokens ADD CONSTRAINT claim_tokens_token_key UNIQUE (token);
CREATE INDEX idx_claim_tokens_token ON claim_tokens(token);

ALTER TABLE claim_tokens DROP CONSTRAINT IF EXISTS claim_tokens_token_hash_key;
ALTER TABLE claim_tokens DROP COLUMN token_sealed;
ALTER TABLE claim_tokens DROP COLUMN token_hash;

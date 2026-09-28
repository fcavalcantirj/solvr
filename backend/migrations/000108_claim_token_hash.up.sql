-- Claim tokens stop being stored in the clear (idx 75 step 5).
--
-- claim_tokens.token held the live claim credential as written: whoever can read the table
-- (a backup, a replica, a support query) could claim any agent whose link is still open, and
-- the claim link is worth becoming the human behind an agent. A row now keeps token_hash, the
-- SHA-256 hex of the token, which is how a claim is looked up, and token_sealed, an
-- AES-256-GCM copy under a key derived from the server secret, so a repeat request for a
-- claim link can hand the same link back without the database holding the token.
--
-- Rows that exist today keep working by hash. They have no sealed copy (the key is not in
-- SQL), so their token cannot be shown again: the next request from that agent replaces the
-- live one with a sealed one. Claim links live four hours, so nothing is lost for longer.

ALTER TABLE claim_tokens ADD COLUMN token_hash CHAR(64);
ALTER TABLE claim_tokens ADD COLUMN token_sealed BYTEA;

UPDATE claim_tokens SET token_hash = encode(sha256(convert_to(token, 'UTF8')), 'hex');

ALTER TABLE claim_tokens ALTER COLUMN token_hash SET NOT NULL;
ALTER TABLE claim_tokens ADD CONSTRAINT claim_tokens_token_hash_key UNIQUE (token_hash);

-- The token column carries the old UNIQUE constraint; dropping it drops that with it.
DROP INDEX IF EXISTS idx_claim_tokens_token;
ALTER TABLE claim_tokens DROP COLUMN token;

-- Restore rooms.token_hash (000073 shape: TEXT NOT NULL). The retired shared tokens were
-- never stored in plaintext, so they cannot be recovered: every room gets a fresh random
-- hash that no plaintext token matches.

ALTER TABLE rooms ADD COLUMN token_hash TEXT;
UPDATE rooms SET token_hash = encode(sha256(gen_random_uuid()::text::bytea), 'hex');
ALTER TABLE rooms ALTER COLUMN token_hash SET NOT NULL;

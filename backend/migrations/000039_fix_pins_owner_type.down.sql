-- Revert owner_type constraint back to original (incorrect) values.
ALTER TABLE pins DROP CONSTRAINT IF EXISTS pins_owner_type_check;
ALTER TABLE pins ADD CONSTRAINT pins_owner_type_check CHECK (owner_type IN ('user', 'agent'));

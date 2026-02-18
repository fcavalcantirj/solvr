-- Fix owner_type constraint: 'user' should be 'human' to match Go model
-- (models.AuthorTypeHuman = "human", not "user").
-- The rest of the codebase (posts table) uses 'human'/'agent', not 'user'/'agent'.

ALTER TABLE pins DROP CONSTRAINT IF EXISTS pins_owner_type_check;
ALTER TABLE pins ADD CONSTRAINT pins_owner_type_check CHECK (owner_type IN ('human', 'agent'));

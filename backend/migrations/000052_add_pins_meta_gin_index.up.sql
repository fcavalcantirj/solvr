-- Add GIN index on pins.meta JSONB column for efficient containment queries.
-- Uses jsonb_path_ops for optimized @> (containment) operator.
CREATE INDEX IF NOT EXISTS idx_pins_meta ON pins USING GIN (meta jsonb_path_ops);

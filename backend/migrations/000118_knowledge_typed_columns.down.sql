-- Reverts 000118: posts timestamps nullable again, replies provenance unbounded. No data is
-- changed either way.
ALTER TABLE replies
    DROP CONSTRAINT IF EXISTS replies_provenance_bounded,
    DROP CONSTRAINT IF EXISTS replies_legacy_pair;
ALTER TABLE posts
    ALTER COLUMN updated_at DROP NOT NULL,
    ALTER COLUMN created_at DROP NOT NULL;

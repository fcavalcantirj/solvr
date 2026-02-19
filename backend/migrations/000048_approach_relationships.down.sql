DROP INDEX IF EXISTS idx_approaches_is_latest;
DROP INDEX IF EXISTS idx_approaches_archived;

ALTER TABLE approaches DROP COLUMN IF EXISTS archived_cid;
ALTER TABLE approaches DROP COLUMN IF EXISTS archived_at;
ALTER TABLE approaches DROP COLUMN IF EXISTS forget_after;
ALTER TABLE approaches DROP COLUMN IF EXISTS is_latest;

DROP INDEX IF EXISTS idx_approach_rel_to;
DROP INDEX IF EXISTS idx_approach_rel_from;
DROP TABLE IF EXISTS approach_relationships;

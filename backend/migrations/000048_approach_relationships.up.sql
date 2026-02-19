-- Approach relationships: tracks updates/extends/derives links between approaches
CREATE TABLE IF NOT EXISTS approach_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    from_approach_id UUID NOT NULL REFERENCES approaches(id),
    to_approach_id UUID NOT NULL REFERENCES approaches(id),
    relation_type VARCHAR(20) NOT NULL CHECK (relation_type IN ('updates', 'extends', 'derives')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(from_approach_id, to_approach_id, relation_type)
);

CREATE INDEX IF NOT EXISTS idx_approach_rel_from ON approach_relationships(from_approach_id);
CREATE INDEX IF NOT EXISTS idx_approach_rel_to ON approach_relationships(to_approach_id);

-- Add versioning and archival columns to approaches
ALTER TABLE approaches ADD COLUMN IF NOT EXISTS is_latest BOOLEAN NOT NULL DEFAULT true;
ALTER TABLE approaches ADD COLUMN IF NOT EXISTS forget_after TIMESTAMPTZ;
ALTER TABLE approaches ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ;
ALTER TABLE approaches ADD COLUMN IF NOT EXISTS archived_cid TEXT;

CREATE INDEX IF NOT EXISTS idx_approaches_archived ON approaches(archived_at) WHERE archived_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_approaches_is_latest ON approaches(problem_id, is_latest) WHERE is_latest = true AND deleted_at IS NULL;

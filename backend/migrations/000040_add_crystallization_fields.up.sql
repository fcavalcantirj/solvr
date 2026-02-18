-- Add crystallization fields to posts table for IPFS snapshot archival.
-- When a solved problem is stable, it gets crystallized (snapshotted) to IPFS.
-- crystallization_cid: IPFS CID of the immutable snapshot
-- crystallized_at: timestamp when the crystallization occurred

ALTER TABLE posts ADD COLUMN IF NOT EXISTS crystallization_cid TEXT;
ALTER TABLE posts ADD COLUMN IF NOT EXISTS crystallized_at TIMESTAMPTZ;

-- Partial index: only index posts that have been crystallized.
-- Allows fast lookup of crystallized content without indexing NULLs.
CREATE INDEX IF NOT EXISTS idx_posts_crystallization_cid
    ON posts(crystallization_cid) WHERE crystallization_cid IS NOT NULL;

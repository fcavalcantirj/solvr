-- Add view_count column to posts table
ALTER TABLE posts ADD COLUMN IF NOT EXISTS view_count INTEGER NOT NULL DEFAULT 0;

-- Create post_views table to track unique views per user
CREATE TABLE IF NOT EXISTS post_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    viewer_type VARCHAR(10) NOT NULL CHECK (viewer_type IN ('human', 'agent', 'anonymous')),
    viewer_id VARCHAR(255), -- NULL for anonymous views
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(post_id, viewer_type, viewer_id)
);

-- Index for fast lookups
CREATE INDEX IF NOT EXISTS idx_post_views_post_id ON post_views(post_id);
CREATE INDEX IF NOT EXISTS idx_post_views_viewer ON post_views(viewer_type, viewer_id);

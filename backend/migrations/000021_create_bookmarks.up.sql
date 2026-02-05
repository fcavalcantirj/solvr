-- Bookmarks table for users to save posts
CREATE TABLE IF NOT EXISTS bookmarks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_type VARCHAR(10) NOT NULL CHECK (user_type IN ('human', 'agent')),
    user_id VARCHAR(255) NOT NULL,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_type, user_id, post_id)
);

-- Index for fast lookups by user
CREATE INDEX IF NOT EXISTS idx_bookmarks_user ON bookmarks(user_type, user_id);

-- Index for checking if a post is bookmarked
CREATE INDEX IF NOT EXISTS idx_bookmarks_post ON bookmarks(post_id);

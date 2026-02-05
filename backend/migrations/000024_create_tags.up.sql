-- Tags table for organizing posts
-- SPEC.md Part 2.2: Tags (max 5 per post)
-- This creates a normalized tags table with usage tracking

CREATE TABLE tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(50) NOT NULL UNIQUE,
    usage_count INT DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Junction table for many-to-many relationship
CREATE TABLE post_tags (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (post_id, tag_id)
);

-- Indexes for efficient queries
CREATE INDEX idx_tags_name ON tags(name);
CREATE INDEX idx_tags_usage ON tags(usage_count DESC);
CREATE INDEX idx_post_tags_post ON post_tags(post_id);
CREATE INDEX idx_post_tags_tag ON post_tags(tag_id);

-- Comments
COMMENT ON TABLE tags IS 'Normalized tags for posts with usage tracking';
COMMENT ON TABLE post_tags IS 'Junction table linking posts to their tags';
COMMENT ON COLUMN tags.usage_count IS 'Number of posts using this tag';

CREATE TABLE follows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    follower_type VARCHAR(10) NOT NULL CHECK (follower_type IN ('agent', 'human')),
    follower_id VARCHAR(255) NOT NULL,
    followed_type VARCHAR(10) NOT NULL CHECK (followed_type IN ('agent', 'human')),
    followed_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(follower_type, follower_id, followed_type, followed_id)
);

CREATE INDEX idx_follows_follower ON follows(follower_type, follower_id);
CREATE INDEX idx_follows_followed ON follows(followed_type, followed_id);

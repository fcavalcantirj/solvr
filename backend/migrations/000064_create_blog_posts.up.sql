CREATE TABLE blog_posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug VARCHAR(255) NOT NULL,
    title VARCHAR(300) NOT NULL,
    body TEXT NOT NULL,
    excerpt VARCHAR(500),
    tags TEXT[],
    cover_image_url TEXT,
    posted_by_type VARCHAR(10) NOT NULL,
    posted_by_id VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    view_count INT DEFAULT 0,
    upvotes INT DEFAULT 0,
    downvotes INT DEFAULT 0,
    read_time_minutes INT DEFAULT 1,
    meta_description VARCHAR(160),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT blog_posts_status_check CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT blog_posts_posted_by_type_check CHECK (posted_by_type IN ('human', 'agent'))
);

CREATE UNIQUE INDEX idx_blog_posts_slug ON blog_posts(slug) WHERE deleted_at IS NULL;
CREATE INDEX idx_blog_posts_status ON blog_posts(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_blog_posts_published ON blog_posts(published_at DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX idx_blog_posts_tags ON blog_posts USING GIN(tags) WHERE deleted_at IS NULL;
CREATE INDEX idx_blog_posts_search ON blog_posts USING GIN(to_tsvector('english', title || ' ' || body)) WHERE deleted_at IS NULL;
CREATE INDEX idx_blog_posts_author ON blog_posts(posted_by_type, posted_by_id) WHERE deleted_at IS NULL;

ALTER TABLE votes DROP CONSTRAINT votes_target_type_check;
ALTER TABLE votes ADD CONSTRAINT votes_target_type_check CHECK (target_type IN ('post', 'answer', 'response', 'approach', 'blog_post'));

-- SEO audit 2026-11-08, Release A.
--
-- 1. page_view_events.source_key: a one-day key for the source of a page view, derived by the
--    API as HMAC-SHA256(WEB_REPORT_SECRET || utc_date, client_ip || user_agent), 16 hex chars.
--    Neither the address nor the user agent is stored, and the key cannot be linked across days.
--    It exists for one rule: more than PageViewBurstThreshold views from one key in one minute
--    are a burst (automation, reason page_view_burst), which no single request could tell.
--    Rows stored before this migration have no key; the operator's legacy rule judges them
--    by the minute's total (reason page_view_burst_legacy).
ALTER TABLE page_view_events ADD COLUMN IF NOT EXISTS source_key TEXT;
CREATE INDEX IF NOT EXISTS idx_page_view_events_source_minute
    ON page_view_events (source_key, occurred_at) WHERE source_key IS NOT NULL;

-- 2. blog_post_covers: the generated cover image of a blog post (one per post), served at
--    GET /v1/blog/{slug}/cover.jpg. The row keeps the model and the prompt that drew it.
CREATE TABLE IF NOT EXISTS blog_post_covers (
    blog_post_id UUID PRIMARY KEY REFERENCES blog_posts(id) ON DELETE CASCADE,
    image BYTEA NOT NULL,
    content_type TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT blog_post_covers_content_type_check CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp'))
);

-- 3. blog_posts.cover_attempts: how many times the cover job tried to draw this post's cover
--    (jobs.BlogCoverJob stops after three failures; the operator route still can).
ALTER TABLE blog_posts ADD COLUMN IF NOT EXISTS cover_attempts INT NOT NULL DEFAULT 0;

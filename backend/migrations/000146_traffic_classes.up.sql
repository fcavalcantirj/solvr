-- Who a counted row came from (SPEC.md 27.6): human, agent, crawler or automation. Rows are
-- flagged, never deleted; the counted_* views are the one place reports read them from.
ALTER TABLE funnel_events ADD COLUMN IF NOT EXISTS traffic_class TEXT NOT NULL DEFAULT 'unclassified';
ALTER TABLE funnel_events ADD COLUMN IF NOT EXISTS traffic_reason TEXT;
ALTER TABLE funnel_events DROP CONSTRAINT IF EXISTS funnel_events_traffic_class_check;
ALTER TABLE funnel_events ADD CONSTRAINT funnel_events_traffic_class_check
    CHECK (traffic_class IN ('unclassified', 'human', 'agent', 'crawler', 'automation'));

ALTER TABLE search_queries ADD COLUMN IF NOT EXISTS traffic_class TEXT NOT NULL DEFAULT 'unclassified';
ALTER TABLE search_queries ADD COLUMN IF NOT EXISTS traffic_reason TEXT;
ALTER TABLE search_queries ADD COLUMN IF NOT EXISTS entry TEXT;
ALTER TABLE search_queries DROP CONSTRAINT IF EXISTS search_queries_traffic_class_check;
ALTER TABLE search_queries ADD CONSTRAINT search_queries_traffic_class_check
    CHECK (traffic_class IN ('unclassified', 'human', 'agent', 'crawler', 'automation'));
ALTER TABLE search_queries DROP CONSTRAINT IF EXISTS search_queries_entry_check;
ALTER TABLE search_queries ADD CONSTRAINT search_queries_entry_check
    CHECK (entry IS NULL OR entry IN ('typed', 'link', 'api'));

ALTER TABLE api_request_events ADD COLUMN IF NOT EXISTS error_code TEXT;
ALTER TABLE api_request_events ADD COLUMN IF NOT EXISTS ua_class TEXT;
ALTER TABLE api_request_events DROP CONSTRAINT IF EXISTS api_request_events_error_code_check;
ALTER TABLE api_request_events ADD CONSTRAINT api_request_events_error_code_check
    CHECK (error_code IS NULL OR error_code ~ '^[A-Z][A-Z0-9_]{0,63}$');
ALTER TABLE api_request_events DROP CONSTRAINT IF EXISTS api_request_events_ua_class_check;
ALTER TABLE api_request_events ADD CONSTRAINT api_request_events_ua_class_check
    CHECK (ua_class IS NULL OR ua_class IN ('browser', 'web_server', 'http_client', 'crawler', 'headless', 'none'));

CREATE TABLE IF NOT EXISTS page_view_events (
    id BIGSERIAL PRIMARY KEY,
    content_group TEXT NOT NULL,
    traffic_class TEXT NOT NULL,
    traffic_reason TEXT,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT page_view_events_traffic_class_check
        CHECK (traffic_class IN ('human', 'agent', 'crawler', 'automation')),
    CONSTRAINT page_view_events_content_group_check
        CHECK (content_group IN ('home', 'connect', 'post', 'room', 'transcript', 'agent', 'user', 'blog', 'docs', 'guide', 'collection', 'account', 'other'))
);
CREATE INDEX IF NOT EXISTS idx_page_view_events_occurred_at ON page_view_events (occurred_at DESC);

CREATE OR REPLACE VIEW counted_funnel_events AS
    SELECT * FROM funnel_events WHERE traffic_class NOT IN ('crawler', 'automation');
CREATE OR REPLACE VIEW counted_search_queries AS
    SELECT * FROM search_queries WHERE traffic_class NOT IN ('crawler', 'automation') AND entry IS DISTINCT FROM 'link';

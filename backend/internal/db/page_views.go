package db

import (
	"context"
	"fmt"
)

// Consent-free page views (SPEC.md 27.6, 27.7).
//
// The site's web server reports each page it serves, and the API keeps one raw row per view:
// the kind of page (content_group), who it came from (traffic_class, traffic_reason), when, and
// the day's source key (source_key, 000148): an HMAC of the address and the user agent that
// changes every UTC day, so one source's views of one day can be counted together and nothing
// else. Never the path, the address, the user agent or a cookie, so a row identifies nobody
// and no consent is asked for. Rows are raw events, never stored aggregates (metric_retention.go):
// every figure is counted from them when it is read.

// PageViewRepository writes page views.
type PageViewRepository struct {
	pool *Pool
}

// NewPageViewRepository creates the page view store.
func NewPageViewRepository(pool *Pool) *PageViewRepository {
	return &PageViewRepository{pool: pool}
}

// Record stores one classified page view of a kind of page. sourceKey is the day's key of
// the view's source (traffic.PageViewSourceKey), stored for the burst rule
// (PageViewBurstThreshold); empty stores NULL, which no rule groups.
func (r *PageViewRepository) Record(ctx context.Context, contentGroup string, label TrafficLabel, sourceKey string) error {
	class, reason := label.columns()
	var key any
	if sourceKey != "" {
		key = sourceKey
	}
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO page_view_events (content_group, traffic_class, traffic_reason, source_key) VALUES ($1, $2, $3, $4)`,
		contentGroup, class, reason, key); err != nil {
		LogQueryError(ctx, "PageViews.Record", "page_view_events", err)
		return fmt.Errorf("record page view: %w", err)
	}
	return nil
}

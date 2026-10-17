package db

import (
	"context"
	"fmt"
)

// Consent-free page views (SPEC.md 27.6, 27.7).
//
// The site's web server reports each page it serves, and the API keeps one raw row per view:
// the kind of page (content_group), who it came from (traffic_class, traffic_reason) and when.
// Never the path, an address, the user agent or a cookie, so a row identifies nobody and no
// consent is asked for. Rows are raw events, never stored aggregates (metric_retention.go):
// every figure is counted from them when it is read.

// PageViewRepository writes page views.
type PageViewRepository struct {
	pool *Pool
}

// NewPageViewRepository creates the page view store.
func NewPageViewRepository(pool *Pool) *PageViewRepository {
	return &PageViewRepository{pool: pool}
}

// Record stores one classified page view of a kind of page.
func (r *PageViewRepository) Record(ctx context.Context, contentGroup string, label TrafficLabel) error {
	class, reason := label.columns()
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO page_view_events (content_group, traffic_class, traffic_reason) VALUES ($1, $2, $3)`,
		contentGroup, class, reason); err != nil {
		LogQueryError(ctx, "PageViews.Record", "page_view_events", err)
		return fmt.Errorf("record page view: %w", err)
	}
	return nil
}

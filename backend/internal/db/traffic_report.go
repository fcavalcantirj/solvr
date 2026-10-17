package db

import (
	"context"
	"fmt"
	"time"
)

// The traffic summary (GET /admin/traffic/summary, SPEC.md 27.6): who a window's traffic came
// from. It reads the raw tables on purpose, the one report that does: it shows what the
// counted_* views leave out and why, beside the consent-free page views and the request log.

// PageViewCount is the page views of one kind of page from one class.
type PageViewCount struct {
	ContentGroup string `json:"content_group"`
	Class        string `json:"traffic_class"`
	Count        int    `json:"count"`
}

// FunnelTrafficCount is one funnel step's rows of one class and reason.
type FunnelTrafficCount struct {
	Event  string `json:"event"`
	Class  string `json:"traffic_class"`
	Reason string `json:"traffic_reason"`
	Count  int    `json:"count"`
}

// SearchTrafficCount is the searches of one class entered one way ("" when it was not recorded).
type SearchTrafficCount struct {
	Class string `json:"traffic_class"`
	Entry string `json:"entry"`
	Count int    `json:"count"`
}

// UAClassCount is the recorded requests from one kind of client ("" before it was recorded).
type UAClassCount struct {
	UAClass string `json:"ua_class"`
	Count   int    `json:"count"`
}

// ErrorCodeCount is how often one error code answered, by response class.
type ErrorCodeCount struct {
	ErrorCode   string `json:"error_code"`
	StatusClass int    `json:"status_class"`
	Count       int    `json:"count"`
}

// TrafficSummary is the traffic of [From, To).
type TrafficSummary struct {
	From      time.Time            `json:"from"`
	To        time.Time            `json:"to"`
	PageViews []PageViewCount      `json:"pageviews"`
	Funnel    []FunnelTrafficCount `json:"funnel"`
	Searches  []SearchTrafficCount `json:"searches"`
	Requests  []UAClassCount       `json:"requests"`
	Errors    []ErrorCodeCount     `json:"errors"`
}

// trafficSummaryTopErrors is how many error codes the summary names.
const trafficSummaryTopErrors = 20

// TrafficReportRepository reads the traffic summary.
type TrafficReportRepository struct {
	pool *Pool
}

// NewTrafficReportRepository creates the traffic summary reader.
func NewTrafficReportRepository(pool *Pool) *TrafficReportRepository {
	return &TrafficReportRepository{pool: pool}
}

// Summary reads the traffic of [from, to).
func (r *TrafficReportRepository) Summary(ctx context.Context, from, to time.Time) (*TrafficSummary, error) {
	s := &TrafficSummary{From: from, To: to, PageViews: []PageViewCount{}, Funnel: []FunnelTrafficCount{},
		Searches: []SearchTrafficCount{}, Requests: []UAClassCount{}, Errors: []ErrorCodeCount{}}

	if err := r.read(ctx, "pageviews", `
		SELECT content_group, traffic_class, count(*)::int FROM page_view_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 GROUP BY 1, 2 ORDER BY 1, 2`,
		[]any{from, to}, func(scan func(...any) error) error {
			var c PageViewCount
			err := scan(&c.ContentGroup, &c.Class, &c.Count)
			s.PageViews = append(s.PageViews, c)
			return err
		}); err != nil {
		return nil, err
	}
	if err := r.read(ctx, "funnel", `
		SELECT event_name, traffic_class, COALESCE(traffic_reason, ''), count(*)::int FROM funnel_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 GROUP BY 1, 2, 3 ORDER BY 1, 2, 3`,
		[]any{from, to}, func(scan func(...any) error) error {
			var c FunnelTrafficCount
			err := scan(&c.Event, &c.Class, &c.Reason, &c.Count)
			s.Funnel = append(s.Funnel, c)
			return err
		}); err != nil {
		return nil, err
	}
	if err := r.read(ctx, "searches", `
		SELECT traffic_class, COALESCE(entry, ''), count(*)::int FROM search_queries
		 WHERE searched_at >= $1 AND searched_at < $2 GROUP BY 1, 2 ORDER BY 1, 2`,
		[]any{from, to}, func(scan func(...any) error) error {
			var c SearchTrafficCount
			err := scan(&c.Class, &c.Entry, &c.Count)
			s.Searches = append(s.Searches, c)
			return err
		}); err != nil {
		return nil, err
	}
	if err := r.read(ctx, "requests", `
		SELECT COALESCE(ua_class, ''), count(*)::int FROM api_request_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 GROUP BY 1 ORDER BY 1`,
		[]any{from, to}, func(scan func(...any) error) error {
			var c UAClassCount
			err := scan(&c.UAClass, &c.Count)
			s.Requests = append(s.Requests, c)
			return err
		}); err != nil {
		return nil, err
	}
	if err := r.read(ctx, "errors", `
		SELECT error_code, status_class::int, count(*)::int FROM api_request_events
		 WHERE occurred_at >= $1 AND occurred_at < $2 AND error_code IS NOT NULL
		 GROUP BY 1, 2 ORDER BY 3 DESC, 1 LIMIT $3`,
		[]any{from, to, trafficSummaryTopErrors}, func(scan func(...any) error) error {
			var c ErrorCodeCount
			err := scan(&c.ErrorCode, &c.StatusClass, &c.Count)
			s.Errors = append(s.Errors, c)
			return err
		}); err != nil {
		return nil, err
	}
	return s, nil
}

// read runs one grouped query and hands each row's scan to row.
func (r *TrafficReportRepository) read(ctx context.Context, what, sql string, args []any, row func(scan func(...any) error) error) error {
	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		LogQueryError(ctx, "TrafficReport."+what, "page_view_events", err)
		return fmt.Errorf("traffic summary %s: %w", what, err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows.Scan); err != nil {
			return fmt.Errorf("scan traffic summary %s: %w", what, err)
		}
	}
	return rows.Err()
}

package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/ops"
)

// The read side of the operations report (spec.json idx 79, GET /admin/ops/slo). It
// returns observations only; every judgement against a target is made in package ops.

// TimelineWriteRoutes are the route templates an accepted timeline write arrives on:
// the canonical entries route and the three adapters that create a timeline entry.
var TimelineWriteRoutes = []string{
	"/v1/rooms/{slug}/entries",
	"/v1/rooms/{slug}/messages",
	"/r/{slug}/message",
	"/r/{slug}/events",
}

// OpsSLORepository reads what the service recorded about itself.
type OpsSLORepository struct {
	pool *Pool
}

// NewOpsSLORepository creates the operations report reader.
func NewOpsSLORepository(pool *Pool) *OpsSLORepository {
	return &OpsSLORepository{pool: pool}
}

// ObserveSLO reads the core service checks, the measured read and timeline-write
// latencies and the search latency in [start, end], and the webhook queue at now.
func (r *OpsSLORepository) ObserveSLO(ctx context.Context, start, end, now time.Time) (ops.SLOObservations, error) {
	obs := ops.SLOObservations{WindowStart: start, WindowEnd: end, Now: now}

	rows, err := r.pool.Query(ctx, `
		SELECT service_name, status, checked_at FROM service_checks
		 WHERE service_name = ANY($1) AND checked_at >= $2 AND checked_at <= $3
		 ORDER BY checked_at`, ops.CoreServices, start, end)
	if err != nil {
		LogQueryError(ctx, "ObserveSLO", "service_checks", err)
		return obs, fmt.Errorf("read core checks: %w", err)
	}
	for rows.Next() {
		var c ops.CoreCheck
		if err := rows.Scan(&c.Service, &c.Status, &c.CheckedAt); err != nil {
			rows.Close()
			return obs, fmt.Errorf("scan core check: %w", err)
		}
		obs.CoreChecks = append(obs.CoreChecks, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return obs, fmt.Errorf("read core checks: %w", err)
	}

	if obs.Read, err = r.p95(ctx, "read", `
		SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), COUNT(*)
		  FROM api_request_events
		 WHERE operation_kind = 'poll' AND status_class = 2 AND duration_ms IS NOT NULL
		   AND occurred_at >= $1 AND occurred_at <= $2`, start, end); err != nil {
		return obs, err
	}
	if obs.TimelineWrite, err = r.p95(ctx, "timeline write", `
		SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), COUNT(*)
		  FROM api_request_events
		 WHERE method = 'POST' AND route_template = ANY($3) AND status_class = 2 AND duration_ms IS NOT NULL
		   AND occurred_at >= $1 AND occurred_at <= $2`, start, end, TimelineWriteRoutes); err != nil {
		return obs, err
	}
	if obs.Search, err = r.p95(ctx, "search", `
		SELECT percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), COUNT(*)
		  FROM search_queries
		 WHERE searched_at >= $1 AND searched_at <= $2`, start, end); err != nil {
		return obs, err
	}

	if obs.Webhooks, err = r.WebhookQueue(ctx, now); err != nil {
		return obs, err
	}
	return obs, nil
}

func (r *OpsSLORepository) p95(ctx context.Context, what, query string, args ...any) (ops.LatencyObservation, error) {
	var obs ops.LatencyObservation
	if err := r.pool.QueryRow(ctx, query, args...).Scan(&obs.P95Ms, &obs.Samples); err != nil {
		LogQueryError(ctx, "ObserveSLO", what, err)
		return obs, fmt.Errorf("read %s p95: %w", what, err)
	}
	return obs, nil
}

// WebhookQueue is the webhook delivery backlog at now: pending deliveries, the due
// ones (next attempt at or before now) and the oldest due instant, and the
// deliveries that failed in the last 24 hours.
func (r *OpsSLORepository) WebhookQueue(ctx context.Context, now time.Time) (ops.QueueObservation, error) {
	var q ops.QueueObservation
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE status = 'pending'),
		       COUNT(*) FILTER (WHERE status = 'pending' AND next_attempt_at <= $1),
		       MIN(next_attempt_at) FILTER (WHERE status = 'pending' AND next_attempt_at <= $1),
		       COUNT(*) FILTER (WHERE status = 'failed' AND last_attempt_at >= $1 - INTERVAL '24 hours')
		  FROM webhook_deliveries`, now).Scan(&q.Pending, &q.Due, &q.OldestDue, &q.FailedLast24h)
	if err != nil {
		LogQueryError(ctx, "WebhookQueue", "webhook_deliveries", err)
		return q, fmt.Errorf("read webhook queue: %w", err)
	}
	return q, nil
}

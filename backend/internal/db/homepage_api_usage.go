package db

import (
	"context"
	"fmt"
	"time"
)

// The aggregate API usage behind the homepage, read for one selected window.
//
// Everything here is CALL VOLUME at the API boundary. Three things it is not,
// and the section that renders it says each one out loud:
//
//   - it is not people. One agent can make ten thousand calls.
//   - it is not independent successful tasks. A create that the domain
//     deduplicated is still a request that arrived.
//   - it is not derived from anything else. A call count comes from recorded
//     requests and from nowhere near a message total, a post total or a page
//     of the website. Before instrumentation started there is no measurement,
//     and the section reports that as unavailable rather than as zero.
//
// Private and public operations are aggregated together and only ever as
// anonymous platform-level counts: nothing in this read can be grouped by
// room, by account or by anything else that identifies a participant, because
// the rows carry none of it.

// APIUsagePulse is one reading of aggregate API usage over one window.
type APIUsagePulse struct {
	Window RoomStatsWindow

	// SuccessfulCalls is every confirmed application request answered with a
	// 2xx in the window. A rejected or failed call is recorded but is not one.
	SuccessfulCalls int

	// The actor breakdown of SuccessfulCalls. The three partition it exactly:
	// a call carried an agent credential, a person's credential, or none.
	AgentCalls     int
	HumanCalls     int
	AnonymousCalls int

	// PassivePolls and WriteAndSearchCalls partition SuccessfulCalls the other
	// way: waiting for something to happen, versus making something happen.
	PassivePolls        int
	WriteAndSearchCalls int

	// RoomKnowledgeOperations is how many DISTINCT room or knowledge
	// operations were exercised — distinct work, not distinct URLs: two route
	// adapters of one operation count once.
	RoomKnowledgeOperations int

	// InstrumentedSince is the earliest recorded request, ever. Nil means
	// nothing has ever been recorded, so every figure above is unavailable
	// rather than zero.
	InstrumentedSince *time.Time

	// Series is successful call volume over the window, oldest bucket first.
	Series []BucketCount
}

// successfulCall is the definition of a call that counts.
const successfulCall = `status_class = 2`

// GetAPIUsagePulse reads the whole API-usage section for one window.
func (r *HomepageRepository) GetAPIUsagePulse(ctx context.Context, window RoomStatsWindow) (APIUsagePulse, error) {
	pulse := APIUsagePulse{Window: window}

	query := `
		SELECT
			COUNT(*) FILTER (WHERE ` + successfulCall + ` AND inside_window),
			COUNT(*) FILTER (WHERE ` + successfulCall + ` AND inside_window AND actor_type = 'agent'),
			COUNT(*) FILTER (WHERE ` + successfulCall + ` AND inside_window AND actor_type = 'human'),
			COUNT(*) FILTER (WHERE ` + successfulCall + ` AND inside_window AND actor_type = 'anonymous'),
			COUNT(*) FILTER (WHERE ` + successfulCall + ` AND inside_window AND operation_kind = 'poll'),
			COUNT(*) FILTER (WHERE ` + successfulCall + ` AND inside_window AND operation_kind IN ('write', 'search')),
			COUNT(DISTINCT operation) FILTER (
				WHERE ` + successfulCall + ` AND inside_window
				  AND operation_family IN ('room', 'knowledge')),
			MIN(occurred_at)
		  FROM (
			SELECT status_class, actor_type, operation_kind, operation_family, operation, occurred_at,
			       occurred_at > NOW() - $1::interval AS inside_window
			  FROM api_request_events
		  ) events
	`

	err := r.pool.QueryRow(ctx, query, window.interval()).Scan(
		&pulse.SuccessfulCalls,
		&pulse.AgentCalls,
		&pulse.HumanCalls,
		&pulse.AnonymousCalls,
		&pulse.PassivePolls,
		&pulse.WriteAndSearchCalls,
		&pulse.RoomKnowledgeOperations,
		&pulse.InstrumentedSince,
	)
	if err != nil {
		LogQueryError(ctx, "GetAPIUsagePulse", "api_request_events", err)
		return pulse, fmt.Errorf("get api usage pulse: %w", err)
	}

	series, err := r.apiCallSeries(ctx, window)
	if err != nil {
		return pulse, err
	}
	pulse.Series = series

	return pulse, nil
}

// apiCallSeries returns exactly window.Buckets buckets, oldest first, with
// zeros filled in for quiet steps so the series never has gaps.
func (r *HomepageRepository) apiCallSeries(ctx context.Context, window RoomStatsWindow) ([]BucketCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($1, occurred_at) AS bucket, COUNT(*)
		  FROM api_request_events
		 WHERE `+successfulCall+`
		   AND occurred_at >= date_trunc($1, NOW()) - ($2::int - 1) * $3::interval
		 GROUP BY bucket
	`, window.BucketUnit, window.Buckets, window.bucketInterval())
	if err != nil {
		LogQueryError(ctx, "apiCallSeries", "api_request_events", err)
		return nil, fmt.Errorf("api call series: %w", err)
	}
	defer rows.Close()

	counts := make(map[time.Time]int, window.Buckets)
	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, fmt.Errorf("scan api usage bucket: %w", err)
		}
		counts[truncateToBucket(bucket.UTC(), window.BucketUnit)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	current := truncateToBucket(time.Now().UTC(), window.BucketUnit)
	step := bucketStep(window.BucketUnit)

	series := make([]BucketCount, 0, window.Buckets)
	for i := window.Buckets - 1; i >= 0; i-- {
		start := current.Add(-time.Duration(i) * step)
		series = append(series, BucketCount{BucketStart: start, Count: counts[start]})
	}
	return series, nil
}

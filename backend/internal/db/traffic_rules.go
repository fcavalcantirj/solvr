package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/traffic"
)

// The behavioural traffic rules (SPEC.md 27.6).
//
// A single request cannot always say who sent it. A connection start can't be judged at
// ingest: its skill fetch arrives 30-59 s later and a copy may come later still, and the
// crawler that walked the tag links sent real browser headers. These rules judge rows by how
// they behaved together, over a window:
//
//	B1 fetched_without_copy  a flow with a post-sourced browser connection_started, an agent's
//	                         skill fetch (agent_fetch) and no starter_prompt_copied: the start
//	                         and the fetch are automation.
//	B2 connect_burst         five or more uncopied post-sourced starts in one clock hour: each
//	                         of them is automation.
//	B3 repeated_query        one address sending one query twenty or more times in one clock
//	                         hour: each of those searches is automation.
//
// One code path runs them: the job every 15 minutes over a trailing window
// (jobs.TrafficClassifyJob) and the operator's backfill (POST /admin/traffic/reclassify),
// which may ask for a dry run. A run is one transaction, recomputed from the rows: B1 and B2
// are undone first for every row of the window they flagged, then applied again, so a copy
// that arrived after a flag gives the flow back to a person (the start) and an agent (its
// fetch). B3 never undoes: a clock hour's count of a query only grows. A run also classifies
// the searches stored before classification existed from their stored user agent.
//
// Rows are flagged, never deleted. Confirmed server steps are never judged.

// trafficClassifyLock names the session advisory lock one run holds, the job's or the
// operator's, on any API instance.
const trafficClassifyLock = "solvr:traffic_classify"

// ConnectBurstThreshold is how many uncopied post-sourced starts in one clock hour make B2.
const ConnectBurstThreshold = 5

// RepeatedQueryThreshold is how many searches of one query from one address in one clock
// hour make B3.
const RepeatedQueryThreshold = 20

// The steps of a run, as TrafficRun.Rules counts the rows each one set.
const (
	RuleStoredUserAgents   = "stored_user_agents"
	RuleReset              = "reset"
	RuleFetchedWithoutCopy = string(traffic.ReasonFetchedWithoutCopy)
	RuleConnectBurst       = string(traffic.ReasonConnectBurst)
	RuleRepeatedQuery      = string(traffic.ReasonRepeatedQuery)
)

// TrafficCount is how many rows of a table carry one class and reason.
type TrafficCount struct {
	Class  string `json:"traffic_class"`
	Reason string `json:"traffic_reason"`
	Count  int    `json:"count"`
}

// TrafficTableCounts is a table's rows in the window by class and reason, before the run and
// after it.
type TrafficTableCounts struct {
	Before []TrafficCount `json:"before"`
	After  []TrafficCount `json:"after"`
}

// TrafficRun is what one run judged and set.
type TrafficRun struct {
	// From is the window's start, rounded down to its clock hour; To is its end.
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Rules is how many rows each step set (a row undone and flagged again counts in both).
	Rules         map[string]int     `json:"rules"`
	FunnelEvents  TrafficTableCounts `json:"funnel_events"`
	SearchQueries TrafficTableCounts `json:"search_queries"`
	DryRun        bool               `json:"dry_run"`
}

// TrafficRules applies the behavioural rules.
type TrafficRules struct {
	pool *Pool
}

// NewTrafficRules creates the rules over a pool.
func NewTrafficRules(pool *Pool) *TrafficRules {
	return &TrafficRules{pool: pool}
}

// TryLock takes the classification lock (see tryAdvisoryLock); ok is false while another
// session holds it. A run is applied only under it.
func (r *TrafficRules) TryLock(ctx context.Context) (unlock func(), ok bool, err error) {
	return tryAdvisoryLock(ctx, r.pool, trafficClassifyLock)
}

// ApplyRules is the job's run: Apply over [from, to), written, with the rows each step set.
func (r *TrafficRules) ApplyRules(ctx context.Context, from, to time.Time) (map[string]int, error) {
	run, err := r.Apply(ctx, from, to, false)
	return run.Rules, err
}

// Apply runs the rules over [from, to), from rounded down to its clock hour, in one
// transaction. With dryRun the transaction is rolled back: the answer says what a run would
// set, and nothing is written.
func (r *TrafficRules) Apply(ctx context.Context, from, to time.Time, dryRun bool) (TrafficRun, error) {
	run := TrafficRun{From: from.UTC().Truncate(time.Hour), To: to.UTC(), DryRun: dryRun, Rules: map[string]int{}}
	if !run.From.Before(run.To) {
		return run, fmt.Errorf("traffic rules: the window [%s, %s) is empty", run.From, run.To)
	}
	tx, err := r.pool.BeginTx(ctx)
	if err != nil {
		return run, err
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			LogQueryError(ctx, "TrafficRules.Rollback", "funnel_events", rbErr)
		}
	}()

	if run.FunnelEvents.Before, err = trafficCounts(ctx, tx, funnelTrafficCountsSQL, run.From, run.To); err != nil {
		return run, err
	}
	if run.SearchQueries.Before, err = trafficCounts(ctx, tx, searchTrafficCountsSQL, run.From, run.To); err != nil {
		return run, err
	}
	if run.Rules[RuleStoredUserAgents], err = classifyStoredSearches(ctx, tx, run.From, run.To); err != nil {
		return run, err
	}
	for _, step := range []struct {
		rule, sql string
		args      []any
	}{
		{RuleReset, resetFunnelFlagsSQL, nil},
		{RuleFetchedWithoutCopy, fetchedWithoutCopySQL, nil},
		{RuleConnectBurst, connectBurstSQL, []any{ConnectBurstThreshold}},
		{RuleRepeatedQuery, repeatedQuerySQL, []any{RepeatedQueryThreshold}},
	} {
		tag, execErr := tx.Exec(ctx, step.sql, append([]any{run.From, run.To}, step.args...)...)
		if execErr != nil {
			LogQueryError(ctx, "TrafficRules."+step.rule, "funnel_events", execErr)
			return run, fmt.Errorf("traffic rule %s: %w", step.rule, execErr)
		}
		run.Rules[step.rule] = int(tag.RowsAffected())
	}
	if run.FunnelEvents.After, err = trafficCounts(ctx, tx, funnelTrafficCountsSQL, run.From, run.To); err != nil {
		return run, err
	}
	if run.SearchQueries.After, err = trafficCounts(ctx, tx, searchTrafficCountsSQL, run.From, run.To); err != nil {
		return run, err
	}
	if dryRun {
		return run, nil // the deferred rollback writes nothing
	}
	if err := tx.Commit(ctx); err != nil {
		return run, fmt.Errorf("traffic rules: commit: %w", err)
	}
	return run, nil
}

const funnelTrafficCountsSQL = `
	SELECT traffic_class, COALESCE(traffic_reason, ''), count(*)::int
	  FROM funnel_events WHERE occurred_at >= $1 AND occurred_at < $2
	 GROUP BY 1, 2 ORDER BY 1, 2`

const searchTrafficCountsSQL = `
	SELECT traffic_class, COALESCE(traffic_reason, ''), count(*)::int
	  FROM search_queries WHERE searched_at >= $1 AND searched_at < $2
	 GROUP BY 1, 2 ORDER BY 1, 2`

func trafficCounts(ctx context.Context, tx Tx, sql string, from, to time.Time) ([]TrafficCount, error) {
	rows, err := tx.Query(ctx, sql, from, to)
	if err != nil {
		LogQueryError(ctx, "TrafficRules.counts", "funnel_events", err)
		return nil, fmt.Errorf("traffic counts: %w", err)
	}
	defer rows.Close()
	out := []TrafficCount{}
	for rows.Next() {
		var c TrafficCount
		if err := rows.Scan(&c.Class, &c.Reason, &c.Count); err != nil {
			return nil, fmt.Errorf("scan traffic count: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// classifyStoredSearches classifies the window's unclassified searches, the ones stored before
// classification existed, from their stored user agent. Their fetch headers were never kept,
// so a browser user agent alone reads as a person (traffic.Signals.HeadersKnown).
func classifyStoredSearches(ctx context.Context, tx Tx, from, to time.Time) (int, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, COALESCE(user_agent, ''), searcher_type FROM search_queries
		 WHERE traffic_class = 'unclassified' AND searched_at >= $1 AND searched_at < $2`, from, to)
	if err != nil {
		LogQueryError(ctx, "TrafficRules.storedSearches", "search_queries", err)
		return 0, fmt.Errorf("read stored searches: %w", err)
	}
	var ids []int64
	var classes, reasons []string
	for rows.Next() {
		var id int64
		var ua, searcherType string
		if scanErr := rows.Scan(&id, &ua, &searcherType); scanErr != nil {
			rows.Close()
			return 0, fmt.Errorf("scan stored search: %w", scanErr)
		}
		class, reason := traffic.Classify(traffic.Signals{
			Surface: traffic.SurfaceAPI, UserAgent: ua, AgentCredential: searcherType == "agent",
		})
		ids, classes, reasons = append(ids, id), append(classes, string(class)), append(reasons, string(reason))
	}
	rows.Close()
	if rowsErr := rows.Err(); rowsErr != nil {
		return 0, rowsErr
	}
	if len(ids) == 0 {
		return 0, nil
	}
	tag, err := tx.Exec(ctx, `
		UPDATE search_queries s SET traffic_class = u.class, traffic_reason = u.reason
		  FROM unnest($1::bigint[], $2::text[], $3::text[]) AS u(id, class, reason)
		 WHERE s.id = u.id AND s.traffic_class = 'unclassified'`, ids, classes, reasons)
	if err != nil {
		LogQueryError(ctx, "TrafficRules.storedSearches", "search_queries", err)
		return 0, fmt.Errorf("classify stored searches: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

// resetFunnelFlagsSQL undoes B1 and B2 in the window before they are applied again: a
// browser step goes back to a person's, an agent's skill fetch to an agent's.
const resetFunnelFlagsSQL = `
	UPDATE funnel_events
	   SET traffic_class = CASE WHEN event_name = 'skill_fetched' THEN 'agent' ELSE 'human' END,
	       traffic_reason = NULL
	 WHERE occurred_at >= $1 AND occurred_at < $2
	   AND traffic_reason IN ('fetched_without_copy', 'connect_burst')`

// postStartSQL is a post-sourced browser connection_started, as alias c.
const postStartSQL = `c.event_name = 'connection_started' AND c.source_channel = 'browser' AND c.source_kind = 'post'`

// fetchedWithoutCopySQL is B1. It flags the flow's post-sourced start, if a person's or
// unclassified, and its agent fetch, if an agent's or unclassified.
const fetchedWithoutCopySQL = `
	UPDATE funnel_events f
	   SET traffic_class = 'automation', traffic_reason = 'fetched_without_copy'
	 WHERE f.occurred_at >= $1 AND f.occurred_at < $2 AND f.flow_id IS NOT NULL
	   AND ((f.event_name = 'connection_started' AND f.source_channel = 'browser' AND f.source_kind = 'post'
	         AND f.traffic_class IN ('human', 'unclassified'))
	     OR (f.event_name = 'skill_fetched' AND f.entry_surface = 'agent_fetch'
	         AND f.traffic_class IN ('agent', 'unclassified')))
	   AND EXISTS (SELECT 1 FROM funnel_events c WHERE c.flow_id = f.flow_id AND ` + postStartSQL + `)
	   AND EXISTS (SELECT 1 FROM funnel_events s WHERE s.flow_id = f.flow_id
	                  AND s.event_name = 'skill_fetched' AND s.entry_surface = 'agent_fetch')
	   AND NOT EXISTS (SELECT 1 FROM funnel_events p WHERE p.flow_id = f.flow_id AND p.event_name = 'starter_prompt_copied')`

// connectBurstSQL is B2. Every uncopied post-sourced start of the hour counts toward the
// burst, whatever its class; only a person's or an unclassified one is flagged. $3 is
// ConnectBurstThreshold.
const connectBurstSQL = `
	UPDATE funnel_events f
	   SET traffic_class = 'automation', traffic_reason = 'connect_burst'
	  FROM (SELECT date_trunc('hour', c.occurred_at, 'UTC') AS h
	          FROM funnel_events c
	         WHERE c.occurred_at >= $1 AND c.occurred_at < $2 AND ` + postStartSQL + `
	           AND NOT EXISTS (SELECT 1 FROM funnel_events p WHERE p.flow_id = c.flow_id AND p.event_name = 'starter_prompt_copied')
	         GROUP BY 1 HAVING count(*) >= $3) burst
	 WHERE date_trunc('hour', f.occurred_at, 'UTC') = burst.h
	   AND f.occurred_at >= $1 AND f.occurred_at < $2
	   AND f.event_name = 'connection_started' AND f.source_channel = 'browser' AND f.source_kind = 'post'
	   AND f.traffic_class IN ('human', 'unclassified')
	   AND NOT EXISTS (SELECT 1 FROM funnel_events p WHERE p.flow_id = f.flow_id AND p.event_name = 'starter_prompt_copied')`

// repeatedQuerySQL is B3. A search with no stored address is never grouped. $3 is
// RepeatedQueryThreshold.
const repeatedQuerySQL = `
	UPDATE search_queries s
	   SET traffic_class = 'automation', traffic_reason = 'repeated_query'
	  FROM (SELECT date_trunc('hour', searched_at, 'UTC') AS h, ip_address, query_normalized
	          FROM search_queries
	         WHERE searched_at >= $1 AND searched_at < $2 AND ip_address IS NOT NULL
	         GROUP BY 1, 2, 3 HAVING count(*) >= $3) rep
	 WHERE s.searched_at >= $1 AND s.searched_at < $2
	   AND date_trunc('hour', s.searched_at, 'UTC') = rep.h
	   AND s.ip_address = rep.ip_address AND s.query_normalized = rep.query_normalized
	   AND s.traffic_class IN ('human', 'agent', 'unclassified')`

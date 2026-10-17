package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The behavioural rules (SPEC.md 27.6) judge rows no single request can: a start can't be
// judged at ingest, because its skill fetch arrives 30-59 s later and a copy may come later
// still. Every test runs on its own scratch database: B2 counts every start of a clock hour.

type trafficFixture struct {
	t    *testing.T
	ctx  context.Context
	pool *Pool
}

func newTrafficFixture(t *testing.T) *trafficFixture {
	t.Helper()
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return &trafficFixture{t: t, ctx: ctx, pool: pool}
}

// start is a post-sourced browser connection_started of its own flow, at at, classed class.
func (f *trafficFixture) start(flow string, at time.Time, class string) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface,
		                           source_kind, source_id, traffic_class, traffic_reason, occurred_at)
		VALUES ($1, 'connection_started', 'browser', 'anonymous', 'connect_page', 'post', $2, $3,
		        CASE WHEN $3 = 'unclassified' THEN NULL ELSE 'browser' END, $4)`,
		flow, uuid.New(), class, at)
	require.NoError(f.t, err)
}

func (f *trafficFixture) fetch(flow string, at time.Time) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface,
		                           traffic_class, traffic_reason, occurred_at)
		VALUES ($1, 'skill_fetched', 'web_server', 'anonymous', 'agent_fetch', 'agent', 'http_client', $2)`, flow, at)
	require.NoError(f.t, err)
}

func (f *trafficFixture) copied(flow string, at time.Time) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface,
		                           traffic_class, traffic_reason, occurred_at)
		VALUES ($1, 'starter_prompt_copied', 'browser', 'anonymous', 'connect_page', 'human', 'browser', $2)`, flow, at)
	require.NoError(f.t, err)
}

func (f *trafficFixture) search(ip, query, ua string, at time.Time) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms,
		                            searcher_type, ip_address, user_agent, searched_at)
		VALUES ($1, $1, 1, 'hybrid', 300, 'anonymous', $2, NULLIF($3, ''), $4)`, query, ip, ua, at)
	require.NoError(f.t, err)
}

// label returns the class and reason of a flow's rows of one step.
func (f *trafficFixture) label(flow, event string) string {
	f.t.Helper()
	var label string
	require.NoError(f.t, f.pool.QueryRow(f.ctx, `
		SELECT string_agg(DISTINCT traffic_class || '/' || COALESCE(traffic_reason, ''), ',')
		  FROM funnel_events WHERE flow_id = $1 AND event_name = $2`, flow, event).Scan(&label))
	return label
}

// state is every row's class and reason, to compare a table before and after a run.
func (f *trafficFixture) state() string {
	f.t.Helper()
	var s string
	require.NoError(f.t, f.pool.QueryRow(f.ctx, `
		SELECT COALESCE((SELECT string_agg(id || ':' || traffic_class || '/' || COALESCE(traffic_reason, ''), ',' ORDER BY id) FROM funnel_events), '')
		    || '|' ||
		       COALESCE((SELECT string_agg(id || ':' || traffic_class || '/' || COALESCE(traffic_reason, ''), ',' ORDER BY id) FROM search_queries), '')`).Scan(&s))
	return s
}

func (f *trafficFixture) apply(from, to time.Time, dryRun bool) TrafficRun {
	f.t.Helper()
	run, err := NewTrafficRules(f.pool).Apply(f.ctx, from, to, dryRun)
	require.NoError(f.t, err)
	return run
}

// The 10-12 shape (measured on production): starts from many posts in one hour, each followed
// 45 s later by an agent's fetch of the skill, and no copy. Every start and every fetch is
// automation, and a flow that was copied in the same hour stays a person's.
func TestTrafficRules_TheMeasuredShapeIsFlagged(t *testing.T) {
	f := newTrafficFixture(t)
	hour := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	for i := 0; i < 17; i++ {
		flow := fmt.Sprintf("f1012_%02d", i)
		at := hour.Add(time.Duration(i*3) * time.Minute)
		f.start(flow, at, "unclassified")
		f.fetch(flow, at.Add(45*time.Second))
	}
	f.start("f1012_copied", hour.Add(20*time.Minute), "human")
	f.copied("f1012_copied", hour.Add(21*time.Minute))
	f.fetch("f1012_copied", hour.Add(22*time.Minute))

	run := f.apply(hour, hour.Add(time.Hour), false)

	for i := 0; i < 17; i++ {
		flow := fmt.Sprintf("f1012_%02d", i)
		assert.Equal(t, "automation/fetched_without_copy", f.label(flow, "connection_started"), flow)
		assert.Equal(t, "automation/fetched_without_copy", f.label(flow, "skill_fetched"), flow)
	}
	assert.Equal(t, "human/browser", f.label("f1012_copied", "connection_started"))
	assert.Equal(t, "agent/http_client", f.label("f1012_copied", "skill_fetched"))
	assert.Equal(t, 34, run.Rules[RuleFetchedWithoutCopy])
	assert.False(t, run.DryRun)
	assert.Contains(t, run.FunnelEvents.After, TrafficCount{Class: "automation", Reason: "fetched_without_copy", Count: 34})
}

// B2: five or more uncopied post-sourced starts in one clock hour are a burst, whatever they
// fetched. Four are not, and starts classed crawler at ingest keep their class.
func TestTrafficRules_ABurstOfUncopiedStarts(t *testing.T) {
	f := newTrafficFixture(t)
	burst := time.Date(2026, 10, 15, 8, 0, 0, 0, time.UTC)
	quiet := burst.Add(time.Hour)
	for i := 0; i < 5; i++ {
		f.start(fmt.Sprintf("burst_%d", i), burst.Add(time.Duration(i*10)*time.Minute), "human")
	}
	for i := 0; i < 4; i++ {
		f.start(fmt.Sprintf("quiet_%d", i), quiet.Add(time.Duration(i*10)*time.Minute), "human")
	}
	_, err := f.pool.Exec(f.ctx, `UPDATE funnel_events SET traffic_class = 'crawler', traffic_reason = 'declared_bot' WHERE flow_id = 'burst_4'`)
	require.NoError(t, err)

	run := f.apply(burst, quiet.Add(time.Hour), false)

	for i := 0; i < 4; i++ {
		assert.Equal(t, "automation/connect_burst", f.label(fmt.Sprintf("burst_%d", i), "connection_started"))
		assert.Equal(t, "human/browser", f.label(fmt.Sprintf("quiet_%d", i), "connection_started"))
	}
	assert.Equal(t, "crawler/declared_bot", f.label("burst_4", "connection_started"))
	assert.Equal(t, 4, run.Rules[RuleConnectBurst])
}

// A copy that arrives after a flow was flagged undoes the flag at the next run: the start goes
// back to a person's, the fetch to an agent's. A burst that a copy brings under five is no
// burst any more.
func TestTrafficRules_ALaterCopyResetsTheFlow(t *testing.T) {
	f := newTrafficFixture(t)
	hour := time.Date(2026, 10, 16, 3, 0, 0, 0, time.UTC)
	f.start("late_copy", hour, "human")
	f.fetch("late_copy", hour.Add(45*time.Second))
	for i := 0; i < 5; i++ {
		f.start(fmt.Sprintf("five_%d", i), hour.Add(time.Duration(10+i)*time.Minute), "human")
	}
	f.apply(hour, hour.Add(time.Hour), false)
	require.Equal(t, "automation/fetched_without_copy", f.label("late_copy", "connection_started"))
	require.Equal(t, "automation/connect_burst", f.label("five_0", "connection_started"))

	f.copied("late_copy", hour.Add(50*time.Minute))
	f.copied("five_0", hour.Add(51*time.Minute))
	f.apply(hour, hour.Add(time.Hour), false)

	assert.Equal(t, "human/", f.label("late_copy", "connection_started"))
	assert.Equal(t, "agent/", f.label("late_copy", "skill_fetched"))
	for i := 0; i < 5; i++ {
		assert.Equal(t, "human/", f.label(fmt.Sprintf("five_%d", i), "connection_started"), "five_%d", i)
	}
}

// B3: one address sending one query twenty or more times in one clock hour is automation, an
// agent's included. Nineteen is not.
func TestTrafficRules_ARepeatedQuery(t *testing.T) {
	f := newTrafficFixture(t)
	hour := time.Date(2026, 10, 13, 3, 0, 0, 0, time.UTC)
	for i := 0; i < 20; i++ {
		f.search("203.0.113.7", "openclaw agent heartbeat", "curl/7.76.1", hour.Add(time.Duration(i)*time.Minute))
	}
	for i := 0; i < 19; i++ {
		f.search("203.0.113.8", "openclaw agent heartbeat", "curl/7.76.1", hour.Add(time.Duration(i)*time.Minute))
	}

	run := f.apply(hour, hour.Add(time.Hour), false)

	var repeated, other int
	require.NoError(t, f.pool.QueryRow(f.ctx, `
		SELECT count(*) FILTER (WHERE host(ip_address) = '203.0.113.7' AND traffic_class = 'automation' AND traffic_reason = 'repeated_query'),
		       count(*) FILTER (WHERE host(ip_address) = '203.0.113.8' AND traffic_class = 'agent' AND traffic_reason = 'http_client')
		  FROM search_queries`).Scan(&repeated, &other))
	assert.Equal(t, 20, repeated)
	assert.Equal(t, 19, other)
	assert.Equal(t, 20, run.Rules[RuleRepeatedQuery])
}

// A search stored before classification existed is classified from its stored user agent, with
// the headers it never kept treated as unknown.
func TestTrafficRules_StoredSearchesAreClassifiedFromTheirUserAgent(t *testing.T) {
	f := newTrafficFixture(t)
	at := time.Date(2026, 10, 12, 10, 0, 0, 0, time.UTC)
	f.search("198.51.100.1", "scraping", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36", at)
	f.search("198.51.100.2", "edge-tts", "Mozilla/5.0 (iPhone; CPU iPhone OS 11_0 like Mac OS X) AppleWebKit/604.1.38 (KHTML, like Gecko) Version/26.5 Mobile/15A372 Safari/604.1", at)
	f.search("198.51.100.3", "symlink", "curl/8.7.1", at)
	f.search("198.51.100.4", "deadlock", "", at)
	f.search("198.51.100.5", "kubernetes", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", at)

	run := f.apply(at, at.Add(time.Hour), false)

	got := map[string]string{}
	rows, err := f.pool.Query(f.ctx, `SELECT query, traffic_class || '/' || COALESCE(traffic_reason, '') FROM search_queries`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var q, l string
		require.NoError(t, rows.Scan(&q, &l))
		got[q] = l
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, map[string]string{
		"scraping":   "human/browser",
		"edge-tts":   "automation/impossible_ua",
		"symlink":    "agent/http_client",
		"deadlock":   "agent/http_client",
		"kubernetes": "crawler/declared_bot",
	}, got)
	assert.Equal(t, 5, run.Rules[RuleStoredUserAgents])
}

// Only the window is judged: a row outside it keeps its class, and a confirmed server step is
// never judged at all.
func TestTrafficRules_OnlyTheWindowAndNeverAServerStep(t *testing.T) {
	f := newTrafficFixture(t)
	hour := time.Date(2026, 10, 12, 11, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		f.start(fmt.Sprintf("outside_%d", i), hour.Add(-2*time.Hour).Add(time.Duration(i)*time.Minute), "human")
	}
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, room_id, occurred_at)
		VALUES ('server_flow', 'room_created', 'server', 'agent', $1, $2)`, uuid.New(), hour.Add(5*time.Minute))
	require.NoError(t, err)

	f.apply(hour, hour.Add(time.Hour), false)

	assert.Equal(t, "human/browser", f.label("outside_0", "connection_started"))
	assert.Equal(t, "unclassified/", f.label("server_flow", "room_created"))
}

// The rules are a function of the rows: a second run over the same window changes nothing.
// A dry run reports what a run would set and writes nothing.
func TestTrafficRules_ARerunChangesNothingAndADryRunWritesNothing(t *testing.T) {
	f := newTrafficFixture(t)
	hour := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 9; i++ {
		flow := fmt.Sprintf("rerun_%d", i)
		f.start(flow, hour.Add(time.Duration(i*5)*time.Minute), "unclassified")
		if i%2 == 0 {
			f.fetch(flow, hour.Add(time.Duration(i*5)*time.Minute+45*time.Second))
		}
	}
	for i := 0; i < 21; i++ {
		f.search("192.0.2.9", "same query", "curl/7.76.1", hour.Add(time.Duration(i)*time.Minute))
	}
	untouched := f.state()

	dry := f.apply(hour, hour.Add(time.Hour), true)
	assert.True(t, dry.DryRun)
	assert.Equal(t, untouched, f.state(), "a dry run writes nothing")

	first := f.apply(hour, hour.Add(time.Hour), false)
	assert.NotEqual(t, untouched, f.state())
	assert.Equal(t, dry.FunnelEvents.After, first.FunnelEvents.After, "the dry run reported what the run set")
	assert.Equal(t, dry.SearchQueries.After, first.SearchQueries.After)
	afterFirst := f.state()

	second := f.apply(hour, hour.Add(time.Hour), false)
	assert.Equal(t, afterFirst, f.state(), "a second run changes nothing")
	assert.Equal(t, second.FunnelEvents.Before, second.FunnelEvents.After)
	assert.Equal(t, second.SearchQueries.Before, second.SearchQueries.After)
}

// B2 and B3 count whole clock hours, so a window that starts inside an hour is read from the
// start of that hour.
func TestTrafficRules_TheWindowStartsOnTheHour(t *testing.T) {
	f := newTrafficFixture(t)
	hour := time.Date(2026, 10, 15, 7, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		f.start(fmt.Sprintf("hour_%d", i), hour.Add(time.Duration(i*5)*time.Minute), "human")
	}
	run := f.apply(hour.Add(40*time.Minute), hour.Add(2*time.Hour), false)
	assert.Equal(t, hour, run.From)
	for i := 0; i < 5; i++ {
		assert.Equal(t, "automation/connect_burst", f.label(fmt.Sprintf("hour_%d", i), "connection_started"))
	}
}

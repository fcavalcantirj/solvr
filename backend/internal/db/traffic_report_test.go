package db

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /admin/traffic/summary reads who the traffic of a window came from (SPEC.md 27.6). It
// reads the raw tables on purpose: it is the one report that shows what the counted_* views
// leave out, and why.
func TestTrafficReport_SummarisesTheWindow(t *testing.T) {
	f := newTrafficFixture(t)
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	in, out := from.Add(9*time.Hour), from.Add(-time.Hour)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := f.pool.Exec(f.ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	for _, v := range []struct {
		at                   time.Time
		group, class, reason string
	}{
		{in, "room", "human", "browser"}, {in, "room", "human", "browser"}, {in, "room", "automation", "no_browser_headers"},
		{in, "home", "crawler", "declared_bot"}, {out, "home", "human", "browser"},
	} {
		exec(`INSERT INTO page_view_events (content_group, traffic_class, traffic_reason, occurred_at) VALUES ($1, $2, $3, $4)`,
			v.group, v.class, v.reason, v.at)
	}
	f.start("summary_a", in, "human")
	exec(`UPDATE funnel_events SET traffic_class = 'automation', traffic_reason = 'connect_burst' WHERE flow_id = 'summary_a'`)
	f.start("summary_b", in, "human")
	f.search("192.0.2.1", "tag", "", in)
	exec(`UPDATE search_queries SET traffic_class = 'human', traffic_reason = 'browser', entry = 'link'`)
	f.search("192.0.2.2", "typed words", "", in)
	exec(`UPDATE search_queries SET traffic_class = 'human', traffic_reason = 'browser', entry = 'typed' WHERE query = 'typed words'`)
	for i, r := range []struct {
		ua, code string
		class    int
	}{{"browser", "", 2}, {"browser", "NOT_FOUND", 4}, {"http_client", "NOT_FOUND", 4}, {"web_server", "INTERNAL_ERROR", 5}} {
		exec(`INSERT INTO api_request_events (request_id, route_template, method, operation, operation_family, operation_kind,
		                                      actor_type, status_class, occurred_at, ua_class, error_code)
		      VALUES ($1, '/v1/posts/{id}', 'GET', 'knowledge.post.read', 'knowledge', 'poll', 'anonymous', $2, $3, $4, NULLIF($5, ''))`,
			uuid.NewString()+string(rune('a'+i)), r.class, in, r.ua, r.code)
	}

	s, err := NewTrafficReportRepository(f.pool).Summary(f.ctx, from, to)
	require.NoError(t, err)

	assert.ElementsMatch(t, []PageViewCount{
		{ContentGroup: "home", Class: "crawler", Count: 1},
		{ContentGroup: "room", Class: "automation", Count: 1},
		{ContentGroup: "room", Class: "human", Count: 2},
	}, s.PageViews)
	assert.ElementsMatch(t, []FunnelTrafficCount{
		{Event: "connection_started", Class: "automation", Reason: "connect_burst", Count: 1},
		{Event: "connection_started", Class: "human", Reason: "browser", Count: 1},
	}, s.Funnel)
	assert.ElementsMatch(t, []SearchTrafficCount{
		{Class: "human", Entry: "link", Count: 1},
		{Class: "human", Entry: "typed", Count: 1},
	}, s.Searches)
	assert.ElementsMatch(t, []UAClassCount{
		{UAClass: "browser", Count: 2}, {UAClass: "http_client", Count: 1}, {UAClass: "web_server", Count: 1},
	}, s.Requests)
	assert.Equal(t, []ErrorCodeCount{
		{ErrorCode: "NOT_FOUND", StatusClass: 4, Count: 2},
		{ErrorCode: "INTERNAL_ERROR", StatusClass: 5, Count: 1},
	}, s.Errors)
}

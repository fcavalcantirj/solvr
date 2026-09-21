package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// What people and agents search for, as the homepage may publish it.
//
// Two separate eligibility rules are asserted here and they must not collapse
// into one:
//
//   - COUNT eligibility decides what is measured. Known automated monitoring
//     is excluded; everything else is counted, including searches recorded
//     before scope was instrumented.
//   - TEXT eligibility decides what may be QUOTED. A term is published only
//     when the search provably ran over public content and the text itself
//     passes db.PublicQueryText.
//
// The suite shares one database and other packages serve real searches while
// these run, so global totals are checked against an independently written
// census taken in the same breath, while term lists are narrowed to this
// fixture's own namespace.

// searchFixture is an isolated slice of search history, namespaced by a prefix
// carried inside the query text itself.
type searchFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *db.Pool
	prefix string
}

func newSearchFixture(t *testing.T, ctx context.Context, pool *db.Pool) *searchFixture {
	t.Helper()
	f := &searchFixture{
		t:      t,
		ctx:    ctx,
		pool:   pool,
		prefix: fmt.Sprintf("sqfix%s", time.Now().Format("150405.000000")[:12]),
	}
	return f
}

// cleanup must be deferred by the test itself, AFTER the pool's own deferred
// close: t.Cleanup would run once the pool is already shut and delete nothing.
func (f *searchFixture) cleanup() {
	f.pool.Exec(f.ctx, `DELETE FROM search_queries WHERE query_normalized LIKE $1`, f.prefix+"%") //nolint:errcheck
}

// record writes one search exactly as handlers.Search writes one.
func (f *searchFixture) record(term, searcherType string, results int, publicScope *bool, userAgent string, ago time.Duration) string {
	f.t.Helper()
	normalized := f.prefix + " " + term
	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO search_queries (
			query, query_normalized, results_count, search_method, duration_ms,
			searcher_type, user_agent, page, searched_at, public_scope
		) VALUES ($1, $1, $2, 'keyword', 12, $3, $4, 1, NOW() - $5::interval, $6)
	`, normalized, results, searcherType, nullableUA(userAgent),
		fmt.Sprintf("%d seconds", int(ago.Seconds())), publicScope)
	require.NoError(f.t, err, "record search %q", normalized)
	return normalized
}

func nullableUA(ua string) any {
	if ua == "" {
		return nil
	}
	return ua
}

// mineTop narrows a term list to this fixture's namespace.
func (f *searchFixture) mineTop(terms []db.SearchTerm) []db.SearchTerm {
	out := make([]db.SearchTerm, 0, len(terms))
	for _, term := range terms {
		if len(term.Query) >= len(f.prefix) && term.Query[:len(f.prefix)] == f.prefix {
			out = append(out, term)
		}
	}
	return out
}

func (f *searchFixture) mineRecent(terms []db.RecentSearchTerm) []db.RecentSearchTerm {
	out := make([]db.RecentSearchTerm, 0, len(terms))
	for _, term := range terms {
		if len(term.Query) >= len(f.prefix) && term.Query[:len(f.prefix)] == f.prefix {
			out = append(out, term)
		}
	}
	return out
}

// searchCensus recomputes the windowed totals from the stated definitions,
// written a different way (make_interval and a correlated NOT EXISTS over the
// monitor patterns instead of ILIKE ANY inside a derived table).
type searchCensus struct {
	eligible, agent, human, anonymous, monitoring, unknownScope int
}

func takeSearchCensus(t *testing.T, ctx context.Context, pool *db.Pool, window db.RoomStatsWindow) searchCensus {
	t.Helper()
	var c searchCensus
	err := pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE NOT mon),
		  COUNT(*) FILTER (WHERE NOT mon AND searcher_type = 'agent'),
		  COUNT(*) FILTER (WHERE NOT mon AND searcher_type = 'human'),
		  COUNT(*) FILTER (WHERE NOT mon AND searcher_type = 'anonymous'),
		  COUNT(*) FILTER (WHERE mon),
		  COUNT(*) FILTER (WHERE NOT mon AND public_scope IS NULL)
		FROM (
		  SELECT sq.searcher_type,
		         sq.public_scope,
		         EXISTS (SELECT 1 FROM unnest($2::text[]) p
		                  WHERE COALESCE(sq.user_agent, '') ILIKE p) AS mon
		    FROM search_queries sq
		   WHERE sq.searched_at >= NOW() - make_interval(secs => $1)
		) s
	`, window.Duration.Seconds(), db.KnownMonitoringAgents).
		Scan(&c.eligible, &c.agent, &c.human, &c.anonymous, &c.monitoring, &c.unknownScope)
	require.NoError(t, err)
	return c
}

func TestSearchPulse_TotalsMatchAnIndependentCensus(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	yes, no := true, false

	f.record("connection pool", "agent", 4, &yes, "", time.Minute)
	f.record("connection pool", "agent", 4, &yes, "", 2*time.Minute)
	f.record("connection pool", "anonymous", 4, &yes, "", 3*time.Minute)
	f.record("family only", "human", 1, &no, "", 4*time.Minute)
	f.record("legacy row", "anonymous", 0, nil, "", 5*time.Minute)
	f.record("uptime probe", "anonymous", 0, &yes, "UptimeRobot/2.0", 6*time.Minute)

	window := db.DefaultRoomStatsWindow()
	pulse, err := db.NewHomepageRepository(pool).GetSearchPulse(ctx, window, 5)
	require.NoError(t, err)

	c := takeSearchCensus(t, ctx, pool, window)
	assert.Equal(t, c.eligible, pulse.Eligible, "eligible total")
	assert.Equal(t, c.agent, pulse.Agent, "agent searches")
	assert.Equal(t, c.human, pulse.Human, "human searches")
	assert.Equal(t, c.anonymous, pulse.Anonymous, "anonymous searches")
	assert.Equal(t, c.monitoring, pulse.Monitoring, "monitoring searches")
	assert.Equal(t, c.unknownScope, pulse.UnknownScope, "unknown-scope searches")

	// The four published breakdowns partition the eligible total exactly.
	assert.Equal(t, pulse.Eligible, pulse.Agent+pulse.Human+pulse.Anonymous,
		"agent + human + anonymous must account for every eligible search")
	assert.Equal(t, window.Value, pulse.Window.Value)
}

func TestSearchPulse_AnonymousIsNeverCountedAsHuman(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	yes := true

	before, err := db.NewHomepageRepository(pool).GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		f.record("unauthenticated visitor", "anonymous", 2, &yes, "", time.Duration(i+1)*time.Minute)
	}

	after, err := db.NewHomepageRepository(pool).GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	assert.Equal(t, before.Human, after.Human,
		"three anonymous searches must not move the human figure")
	assert.GreaterOrEqual(t, after.Anonymous-before.Anonymous, 3)
}

func TestSearchPulse_KnownMonitoringIsSeparatedNotCounted(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	yes := true

	before, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	for i := 0; i < 4; i++ {
		f.record("status ping", "anonymous", 1, &yes, "UptimeRobot/2.0; http://uptimerobot.com/", time.Duration(i+1)*time.Minute)
	}

	after, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	assert.Equal(t, before.Eligible, after.Eligible, "monitoring must not enter the default view")
	assert.Equal(t, before.Anonymous, after.Anonymous, "monitoring must not enter the anonymous figure")
	assert.Equal(t, 4, after.Monitoring-before.Monitoring, "monitoring must be reported separately")

	assert.Empty(t, f.mineTop(after.Top), "a monitored term must not reach the top table")
	assert.Empty(t, f.mineRecent(after.Recent), "a monitored term must not reach the recent list")
}

func TestSearchPulse_AGenericClientIsNotAssumedToBeAMonitor(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	yes := true

	before, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	// Go-http-client is what every SDK and every agent sends. Treating it as
	// monitoring would silently delete real agent activity from the page.
	for i := 0; i < 2; i++ {
		f.record("sdk client search", "agent", 3, &yes, "Go-http-client/1.1", time.Duration(i+1)*time.Minute)
	}

	after, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)
	assert.Equal(t, before.Monitoring, after.Monitoring, "a generic HTTP client is not known monitoring")
	assert.Equal(t, 2, after.Agent-before.Agent)
}

func TestSearchPulse_PublishesOnlyPublicScopeTerms(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	yes, no := true, false

	public := f.record("public term", "agent", 5, &yes, "", time.Minute)
	f.record("public term", "agent", 5, &yes, "", 2*time.Minute)
	f.record("family term", "human", 1, &no, "", 3*time.Minute)
	f.record("family term", "human", 1, &no, "", 4*time.Minute)
	f.record("legacy term", "anonymous", 1, nil, "", 5*time.Minute)
	f.record("legacy term", "anonymous", 1, nil, "", 6*time.Minute)

	pulse, err := db.NewHomepageRepository(pool).GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	top := f.mineTop(pulse.Top)
	require.Len(t, top, 1, "only the public-scope term may be published: %+v", top)
	assert.Equal(t, public, top[0].Query)
	assert.Equal(t, 2, top[0].Count)
	assert.Equal(t, 2, top[0].WithResults)

	recent := f.mineRecent(pulse.Recent)
	require.Len(t, recent, 1)
	assert.Equal(t, public, recent[0].Query)
	assert.Equal(t, "agent", recent[0].SearcherType)
}

func TestSearchPulse_WithResultsCountsOnlyProductiveSearches(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	yes := true

	term := f.record("half productive", "agent", 7, &yes, "", time.Minute)
	f.record("half productive", "agent", 0, &yes, "", 2*time.Minute)
	f.record("half productive", "anonymous", 0, &yes, "", 3*time.Minute)

	pulse, err := db.NewHomepageRepository(pool).GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	top := f.mineTop(pulse.Top)
	require.Len(t, top, 1)
	assert.Equal(t, term, top[0].Query)
	assert.Equal(t, 3, top[0].Count)
	assert.Equal(t, 1, top[0].WithResults, "two of the three searches found nothing")
}

func TestSearchPulse_ASingleSearchIsNeverPublished(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	yes := true

	f.record("searched once only", "agent", 3, &yes, "", time.Minute)

	pulse, err := db.NewHomepageRepository(pool).GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	assert.Empty(t, f.mineTop(pulse.Top), "one search belongs to one searcher")
	assert.Empty(t, f.mineRecent(pulse.Recent))
}

func TestSearchPulse_WithholdsUnsafeTextAndSaysHowMany(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	yes := true

	before, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	for i := 0; i < 2; i++ {
		f.record("sk-abcd1234abcd1234abcd1234", "agent", 1, &yes, "", time.Duration(i+1)*time.Minute)
		f.record("mail someone@example.com bounced", "agent", 1, &yes, "", time.Duration(i+3)*time.Minute)
	}

	after, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	assert.Empty(t, f.mineTop(after.Top), "credential- and personal-shaped terms are never published")
	assert.Empty(t, f.mineRecent(after.Recent))
	assert.GreaterOrEqual(t, after.WithheldTerms-before.WithheldTerms, 2,
		"the withheld terms must be counted so the page can say so")
}

func TestSearchPulse_SeriesCoversTheWindowAndExcludesMonitoring(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	yes := true

	for _, window := range db.RoomStatsWindows {
		pulse, err := repo.GetSearchPulse(ctx, window, 5)
		require.NoError(t, err)
		assert.Len(t, pulse.Series, window.Buckets, "window %s", window.Value)

		total := 0
		for _, bucket := range pulse.Series {
			total += bucket.Count
		}
		assert.Equal(t, pulse.Eligible, total,
			"window %s: the series must add up to the eligible total it illustrates", window.Value)
	}

	before, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)
	f.record("probe", "anonymous", 0, &yes, "Pingdom.com_bot_version_1.4", time.Minute)
	after, err := repo.GetSearchPulse(ctx, db.DefaultRoomStatsWindow(), 5)
	require.NoError(t, err)

	beforeTotal, afterTotal := 0, 0
	for _, b := range before.Series {
		beforeTotal += b.Count
	}
	for _, b := range after.Series {
		afterTotal += b.Count
	}
	assert.Equal(t, beforeTotal, afterTotal, "monitoring must not appear in the chart either")
}

func TestSearchPulse_WindowNarrowsWhatIsCounted(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newSearchFixture(t, ctx, pool)
	defer f.cleanup()
	repo := db.NewHomepageRepository(pool)
	yes := true

	day, ok := db.RoomStatsWindowByValue("24h")
	require.True(t, ok)
	week, ok := db.RoomStatsWindowByValue("7d")
	require.True(t, ok)

	// A wide limit on both reads: the question is whether the term is IN the
	// window at all, and other packages serve real searches into this same
	// database while this runs, so a short list proves nothing by its absence.
	beforeWeek, err := repo.GetSearchPulse(ctx, week, 200)
	require.NoError(t, err)

	f.record("three days back", "agent", 2, &yes, "", 72*time.Hour)
	f.record("three days back", "agent", 2, &yes, "", 73*time.Hour)

	afterDay, err := repo.GetSearchPulse(ctx, day, 200)
	require.NoError(t, err)
	afterWeek, err := repo.GetSearchPulse(ctx, week, 200)
	require.NoError(t, err)

	assert.Empty(t, f.mineTop(afterDay.Top), "a 3-day-old search is outside 24 hours")
	assert.Empty(t, f.mineRecent(afterDay.Recent))

	weekTop := f.mineTop(afterWeek.Top)
	require.Len(t, weekTop, 1, "and inside 7 days")
	assert.Equal(t, 2, weekTop[0].Count)
	require.Len(t, f.mineRecent(afterWeek.Recent), 1)

	// Ambient traffic can only ADD to a global total, so the week total must
	// have grown by at least the two rows written here.
	assert.GreaterOrEqual(t, afterWeek.Eligible-beforeWeek.Eligible, 2)
}

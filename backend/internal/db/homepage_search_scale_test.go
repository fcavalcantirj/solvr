package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The search section reads every search in its window (spec.json idx 77 step 1: search time
// windows, measured on representative data). Measured on HEAD before this change (slice 12
// spike, 2M searches over 180 days, 333k in the 30-day window, 166 user agents): one 30-day
// GetSearchPulse took ~9.8 s, and the access path was not the cost — reading the window's rows
// took ~107 ms. The cost was the monitor test, COALESCE(user_agent, '') ILIKE ANY(18 patterns):
// ILIKE lower()s the text and the pattern once per pattern per row, and the totals statement
// evaluated it once per FILTER clause (6x per row, 6.4 s). The reads must return exactly what
// the per-search definitions say, in bounded work per search.

// seedSearchMonth writes 300k searches 10 s apart across 166 user agents (NULL, generic clients,
// browsers, known monitors in mixed case): 250k inside the last 29 days and 50k older than 32
// days, so no search ages out of a 30-day window while a test compares two reads of it.
func seedSearchMonth(ctx context.Context, t *testing.T, pool *Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms,
		                            searcher_type, user_agent, page, searched_at, public_scope)
		SELECT q.term, q.term, g % 5, 'hybrid', 20, (ARRAY['agent','human','anonymous'])[1 + g % 3],
		       CASE WHEN g % 20 = 0 THEN (ARRAY['UptimeRobot/2.0 (+http://www.uptimerobot.com/)',
		                                       'Pingdom.com_bot_version_1.4_(http://www.pingdom.com/)',
		                                       'kube-probe/1.29', 'Datadog/Synthetics', 'GoogleHC/1.0',
		                                       'Mozilla/5.0 (compatible; BetterUptime Bot)'])[1 + (g / 20) % 6]
		            WHEN g % 10 IN (1, 2, 3) THEN NULL
		            WHEN g % 10 = 4 THEN 'python-requests/2.' || ((g / 10) % 40)
		            WHEN g % 10 = 5 THEN 'Go-http-client/1.1'
		            WHEN g % 10 = 6 THEN 'curl/8.' || ((g / 10) % 10) || '.0'
		            WHEN g % 10 = 7 THEN 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 '
		                                 || '(KHTML, like Gecko) Chrome/1' || ((g / 10) % 60) || '.0.0.0 Safari/537.36'
		            WHEN g % 10 = 8 THEN 'solvr-mcp/0.' || ((g / 10) % 20) || ' node/22'
		            ELSE 'Mozilla/5.0 (X11; Linux x86_64; rv:1' || ((g / 10) % 30) || '.0) Gecko/20100101 Firefox/1'
		                 || ((g / 10) % 30) || '.0' END,
		       1, CASE WHEN g <= 250000 THEN NOW() - g * INTERVAL '10 seconds'
		               ELSE NOW() - INTERVAL '32 days' - (g - 250000) * INTERVAL '10 seconds' END,
		       CASE WHEN g % 7 = 0 THEN NULL WHEN g % 7 = 1 THEN FALSE ELSE TRUE END
		  FROM generate_series(1, 300000) g
		 CROSS JOIN LATERAL (SELECT CASE WHEN g % 3 = 0
		                                 THEN (ARRAY['contact ops@example.com', 'pgx pool exhausted',
		                                             'context deadline exceeded', 'hnsw index build',
		                                             'room token expired', 'agent handshake', 'ivfflat recall',
		                                             'sse reconnect', 'rate limit 429'])[1 + (g / 3) % 9]
		                                 ELSE 'term ' || (g % 4000) END AS term) q`)
	require.NoError(t, err, "seed a month of searches")
	_, err = pool.Exec(ctx, `VACUUM ANALYZE search_queries`)
	require.NoError(t, err, "vacuum analyze the seeded searches")
}

// expectedSearchPulse computes the section from the per-search definitions: a search is known
// monitoring when its user agent ILIKEs any KnownMonitoringAgents pattern; a term is a
// candidate when it was searched at least twice in public scope by non-monitoring callers.
func expectedSearchPulse(ctx context.Context, t *testing.T, pool *Pool, window RoomStatsWindow, limit int) SearchPulse {
	t.Helper()
	want := SearchPulse{Window: window}
	const monitored = `EXISTS (SELECT 1 FROM unnest($2::text[]) p WHERE COALESCE(sq.user_agent, '') ILIKE p)`
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE NOT mon),
		       COUNT(*) FILTER (WHERE NOT mon AND searcher_type = 'agent'),
		       COUNT(*) FILTER (WHERE NOT mon AND searcher_type = 'human'),
		       COUNT(*) FILTER (WHERE NOT mon AND searcher_type = 'anonymous'),
		       COUNT(*) FILTER (WHERE mon),
		       COUNT(*) FILTER (WHERE NOT mon AND public_scope IS NULL)
		  FROM (SELECT sq.searcher_type, sq.public_scope, `+monitored+` AS mon
		          FROM search_queries sq WHERE sq.searched_at >= NOW() - make_interval(secs => $1)) s`,
		window.Duration.Seconds(), KnownMonitoringAgents).Scan(
		&want.Eligible, &want.Agent, &want.Human, &want.Anonymous, &want.Monitoring, &want.UnknownScope))

	rows, err := pool.Query(ctx, `
		SELECT date_trunc($3, sq.searched_at), COUNT(*)
		  FROM search_queries sq
		 WHERE sq.searched_at >= date_trunc($3, NOW()) - make_interval(secs => $1) AND NOT `+monitored+`
		 GROUP BY 1`, (time.Duration(window.Buckets-1) * bucketStep(window.BucketUnit)).Seconds(),
		KnownMonitoringAgents, window.BucketUnit)
	require.NoError(t, err)
	counts := map[time.Time]int{}
	for rows.Next() {
		var bucket time.Time
		var n int
		require.NoError(t, rows.Scan(&bucket, &n))
		counts[truncateToBucket(bucket.UTC(), window.BucketUnit)] = n
	}
	require.NoError(t, rows.Err())
	rows.Close()
	current := truncateToBucket(time.Now().UTC(), window.BucketUnit)
	for i := window.Buckets - 1; i >= 0; i-- {
		start := current.Add(-time.Duration(i) * bucketStep(window.BucketUnit))
		want.Series = append(want.Series, BucketCount{BucketStart: start, Count: counts[start]})
	}

	published := true
	rows, err = pool.Query(ctx, `
		SELECT query_normalized, COUNT(*), COUNT(*) FILTER (WHERE results_count > 0)
		  FROM search_queries sq
		 WHERE sq.searched_at >= NOW() - make_interval(secs => $1) AND sq.public_scope AND NOT `+monitored+`
		 GROUP BY query_normalized HAVING COUNT(*) >= 2
		 ORDER BY 2 DESC, 1`, window.Duration.Seconds(), KnownMonitoringAgents)
	require.NoError(t, err)
	topWithheld, seen := 0, 0
	want.Top = []SearchTerm{}
	for rows.Next() && seen < limit+searchTermOverfetch {
		seen++
		var term SearchTerm
		require.NoError(t, rows.Scan(&term.Query, &term.Count, &term.WithResults))
		if PublicQueryText(term.Query, &published) != PublicQueryOK {
			topWithheld++
		} else if len(want.Top) < limit {
			want.Top = append(want.Top, term)
		}
	}
	rows.Close()

	// The latest search of each candidate term, newest first (timestamps are distinct).
	rows, err = pool.Query(ctx, `
		SELECT DISTINCT ON (query_normalized) query_normalized, searcher_type, results_count, searched_at,
		       COUNT(*) OVER (PARTITION BY query_normalized)
		  FROM search_queries sq
		 WHERE sq.searched_at >= NOW() - make_interval(secs => $1) AND sq.public_scope AND NOT `+monitored+`
		 ORDER BY query_normalized, searched_at DESC`, window.Duration.Seconds(), KnownMonitoringAgents)
	require.NoError(t, err)
	var latest []RecentSearchTerm
	for rows.Next() {
		var term RecentSearchTerm
		require.NoError(t, rows.Scan(&term.Query, &term.SearcherType, &term.ResultsCount, &term.LastSearched, &term.Occurrences))
		if term.Occurrences >= searchTermMinOccurrences {
			latest = append(latest, term)
		}
	}
	require.NoError(t, rows.Err())
	rows.Close()
	for i := range latest { // newest first; a handful of candidates, insertion sort is enough
		for j := i; j > 0 && latest[j].LastSearched.After(latest[j-1].LastSearched); j-- {
			latest[j], latest[j-1] = latest[j-1], latest[j]
		}
	}
	recentWithheld := 0
	want.Recent = []RecentSearchTerm{}
	for i := 0; i < len(latest) && i < limit+searchTermOverfetch; i++ {
		if PublicQueryText(latest[i].Query, &published) != PublicQueryOK {
			recentWithheld++
		} else if len(want.Recent) < limit {
			want.Recent = append(want.Recent, latest[i])
		}
	}
	want.WithheldTerms = max(topWithheld, recentWithheld)
	return want
}

// readFloor is the cheapest possible read of the window: the same rows and columns, no
// classification. Its time is the yardstick, so the bound holds on a slow machine too.
func readFloor(ctx context.Context, t *testing.T, pool *Pool, window RoomStatsWindow) time.Duration {
	t.Helper()
	best := time.Duration(1<<63 - 1)
	for i := 0; i < 3; i++ {
		start := time.Now()
		var a, b, c, d int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COUNT(*) FILTER (WHERE searcher_type = 'agent'), COUNT(user_agent), COUNT(public_scope),
			       COUNT(*) FILTER (WHERE results_count > 0 AND query_normalized <> '')
			  FROM search_queries WHERE searched_at >= NOW() - $1::interval`, window.interval()).Scan(&a, &b, &c, &d))
		best = min(best, time.Since(start))
	}
	return best
}

// searchPulseWorkBound is how many window reads one GetSearchPulse may cost. It runs four
// statements over the window: measured ~13 reads here after this change (281-301 ms against a
// 23 ms read), ~450 before it (9.3-10.4 s).
const searchPulseWorkBound = 30

func TestSearchPulse_AMonthOfSearchesAtGrowthVolumeMatchesTheDefinitionsInBoundedWork(t *testing.T) {
	scratch, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	seedSearchMonth(ctx, t, scratch)

	// One connection, so the 6th call onwards runs on the plan the server caches for these
	// prepared statements, as it does on a long-lived production connection.
	pool, err := NewPool(ctx, scratch.Config().ConnString(), func(c *pgxpool.Config) {
		c.MaxConns, c.MinConns = 1, 1
	})
	require.NoError(t, err)
	defer pool.Close()
	repo := NewHomepageRepository(pool)
	window, ok := RoomStatsWindowByValue("30d")
	require.True(t, ok)

	floor := readFloor(ctx, t, pool, window)
	slowest := time.Duration(0)
	for call := 1; call <= 7; call++ {
		start := time.Now()
		got, err := repo.GetSearchPulse(ctx, window, 5)
		took := time.Since(start)
		require.NoError(t, err)
		slowest = max(slowest, took)
		t.Logf("call %d: %s (window read floor %s)", call, took.Round(time.Millisecond), floor.Round(time.Millisecond))
		if call == 1 || call == 7 {
			want := expectedSearchPulse(ctx, t, pool, window, 5)
			require.Greater(t, want.Eligible, 200000, "the window holds the seeded month")
			require.Greater(t, want.Monitoring, 10000, "and its monitors")
			require.Positive(t, want.WithheldTerms, "and a hot term the text policy withholds")
			require.Equal(t, want, got, "call %d must equal the per-search definitions", call)
		}
	}
	require.LessOrEqual(t, slowest, searchPulseWorkBound*floor, fmt.Sprintf(
		"a 30-day search section must cost at most %d window reads: slowest call %s, one read %s",
		searchPulseWorkBound, slowest.Round(time.Millisecond), floor.Round(time.Millisecond)))
}

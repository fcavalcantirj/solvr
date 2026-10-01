package db

import (
	"context"
	"fmt"
	"time"
)

// What people and agents search for, as a public page may show it.
//
// Two eligibility rules live here and they are deliberately different:
//
//   - COUNT eligibility decides what is MEASURED. Known automated monitoring
//     is excluded, because a robot checking that search still answers is not
//     somebody looking for something. Everything else is counted — including
//     searches recorded before scope was instrumented, whose eligibility is
//     unknown. They are a count and nothing else.
//   - TEXT eligibility decides what may be QUOTED. A term reaches the page
//     only when the search PROVABLY ran over public content (public_scope) and
//     the text itself passes PublicQueryText. Everything else is withheld, and
//     the number withheld is reported so the page can say so rather than
//     quietly shrinking.
//
// On top of both, a term must have been searched more than once. A single
// search belongs to a single searcher, and publishing it would publish them.

// searchTermMinOccurrences is the privacy floor on a published term.
const searchTermMinOccurrences = 2

// searchTermOverfetch is how many extra grouped terms to read before the text
// policy runs in Go. The policy cannot be expressed in SQL, so the read has to
// be able to lose rows and still fill the page.
const searchTermOverfetch = 6

// KnownMonitoringAgents is the allow-list of User-Agent patterns (SQL ILIKE)
// belonging to automated monitoring Solvr KNOWS about.
//
// It is an allow-list of NAMED monitors on purpose. Guessing at "looks like a
// bot" would quietly delete real agent traffic from the page: every Solvr SDK,
// the CLI and most agents send a generic client string such as
// "Go-http-client/1.1", and that is exactly the activity this page exists to
// show. A monitor has to be recognised to be separated.
var KnownMonitoringAgents = []string{
	"%uptimerobot%",
	"%pingdom%",
	"%betteruptime%",
	"%betterstack%",
	"%statuscake%",
	"%site24x7%",
	"%datadog%",
	"%checkly%",
	"%newrelic%",
	"%pingly%",
	"%hetrixtool%",
	"%gtmetrix%",
	"%google-cloud-monitoring%",
	"%googlehc/%",
	"%kube-probe/%",
	"%elb-healthchecker%",
	"%amazoncloudfront%",
	"%solvr-healthcheck%",
}

// SearchTerm is one published term and how often it was searched.
type SearchTerm struct {
	// Query is the normalised term. It has passed PublicQueryText.
	Query string
	// Count is how many publishable searches used it in the window.
	Count int
	// WithResults is how many of those returned at least one result.
	WithResults int
}

// RecentSearchTerm is one published term at its most recent search.
type RecentSearchTerm struct {
	Query string
	// SearcherType is what the request RECORDED: "agent", "human" or
	// "anonymous". Anonymous is its own answer, never folded into human.
	SearcherType string
	Occurrences  int
	ResultsCount int
	LastSearched time.Time
}

// SearchPulse is one reading of search activity over one window.
type SearchPulse struct {
	Window RoomStatsWindow

	// The counted view. Monitoring is excluded from Eligible and from all
	// three breakdowns; the breakdowns partition Eligible exactly.
	Eligible  int
	Agent     int
	Human     int
	Anonymous int

	// Monitoring is the separated figure: known automated monitoring, counted
	// so it can be stated, never mixed into the view above.
	Monitoring int

	// UnknownScope is how many eligible searches were recorded before scope
	// was instrumented. They are counted; their text is never published.
	UnknownScope int

	// WithheldTerms is how many terms cleared the occurrence floor but were
	// held back by the text policy.
	WithheldTerms int

	Series []BucketCount
	Top    []SearchTerm
	Recent []RecentSearchTerm
}

// GetSearchPulse reads the whole search section for one window.
func (r *HomepageRepository) GetSearchPulse(ctx context.Context, window RoomStatsWindow, limit int) (SearchPulse, error) {
	if window.Value == "" {
		window = DefaultRoomStatsWindow()
	}
	if limit <= 0 {
		limit = 5
	}

	pulse := SearchPulse{Window: window}

	monitored, err := r.searchTotals(ctx, window, &pulse)
	if err != nil {
		return pulse, err
	}

	series, err := r.searchSeries(ctx, window, monitored)
	if err != nil {
		return pulse, err
	}
	pulse.Series = series

	top, topWithheld, err := r.topSearchTerms(ctx, window, limit, monitored)
	if err != nil {
		return pulse, err
	}
	pulse.Top = top

	recent, recentWithheld, err := r.recentSearchTerms(ctx, window, limit, monitored)
	if err != nil {
		return pulse, err
	}
	pulse.Recent = recent

	// A term withheld from both lists is one withheld term, not two.
	pulse.WithheldTerms = topWithheld
	if recentWithheld > topWithheld {
		pulse.WithheldTerms = recentWithheld
	}

	return pulse, nil
}

// monitoredExpr is the single definition of "this was known monitoring".
// COALESCE matters: user_agent is NULL for most callers, and NULL ILIKE ANY
// is NULL, which would drop every such row out of the count entirely.
//
// It is only ever applied to DISTINCT user agents (searchTotals), never per
// search: ILIKE ANY lowers the text and every pattern again for each pattern,
// and per search it was nearly all of a 30-day read at growth volume (9.8 s of
// a ~0.1 s window read, spec.json idx 77 slice 12).
const monitoredExpr = `(COALESCE(user_agent, '') ILIKE ANY($2::text[]))`

// notMonitoredExpr keeps a search whose user agent searchTotals did not
// classify as known monitoring. $2 is that list of exact agent strings, so this
// is one hashed lookup per search. An agent string first seen after
// searchTotals read the window is not on the list for that one read.
const notMonitoredExpr = `COALESCE(user_agent, '') NOT IN (SELECT unnest($2::text[]))`

// searchTotals reads the counted view in one pass and returns the user agents
// it classified as known monitoring. Searches are grouped per user agent
// first, so monitoredExpr runs once per agent string in the window.
func (r *HomepageRepository) searchTotals(ctx context.Context, window RoomStatsWindow, pulse *SearchPulse) ([]string, error) {
	var monitored []string
	err := r.pool.QueryRow(ctx, `
		SELECT
		  COALESCE(SUM(n) FILTER (WHERE NOT monitored), 0)::bigint,
		  COALESCE(SUM(n) FILTER (WHERE NOT monitored AND searcher_type = 'agent'), 0)::bigint,
		  COALESCE(SUM(n) FILTER (WHERE NOT monitored AND searcher_type = 'human'), 0)::bigint,
		  COALESCE(SUM(n) FILTER (WHERE NOT monitored AND searcher_type = 'anonymous'), 0)::bigint,
		  COALESCE(SUM(n) FILTER (WHERE monitored), 0)::bigint,
		  COALESCE(SUM(n) FILTER (WHERE NOT monitored AND unknown_scope), 0)::bigint,
		  COALESCE(array_agg(DISTINCT COALESCE(user_agent, '')) FILTER (WHERE monitored), '{}')::text[]
		  FROM (
		    SELECT user_agent, searcher_type, unknown_scope, n, `+monitoredExpr+` AS monitored
		      FROM (
		        SELECT user_agent, searcher_type, public_scope IS NULL AS unknown_scope, COUNT(*) AS n
		          FROM search_queries
		         WHERE searched_at >= NOW() - $1::interval
		         GROUP BY user_agent, searcher_type, public_scope IS NULL
		      ) per_agent
		  ) s
	`, window.interval(), KnownMonitoringAgents).Scan(
		&pulse.Eligible, &pulse.Agent, &pulse.Human, &pulse.Anonymous,
		&pulse.Monitoring, &pulse.UnknownScope, &monitored,
	)
	if err != nil {
		LogQueryError(ctx, "searchTotals", "search_queries", err)
		return nil, fmt.Errorf("search totals: %w", err)
	}
	return monitored, nil
}

// searchSeries returns exactly window.Buckets buckets, oldest first, zeros
// filled in. It uses the same eligibility as the totals, so the chart and the
// numbers above it can never tell different stories.
func (r *HomepageRepository) searchSeries(ctx context.Context, window RoomStatsWindow, monitored []string) ([]BucketCount, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT date_trunc($1, searched_at) AS bucket, COUNT(*)
		  FROM search_queries
		 WHERE searched_at >= date_trunc($1, NOW()) - ($3::int - 1) * $4::interval
		   AND `+notMonitoredExpr+`
		 GROUP BY bucket
	`, window.BucketUnit, monitored, window.Buckets, window.bucketInterval())
	if err != nil {
		LogQueryError(ctx, "searchSeries", "search_queries", err)
		return nil, fmt.Errorf("search series: %w", err)
	}
	defer rows.Close()

	counts := make(map[time.Time]int, window.Buckets)
	for rows.Next() {
		var bucket time.Time
		var count int
		if err := rows.Scan(&bucket, &count); err != nil {
			return nil, fmt.Errorf("scan search series bucket: %w", err)
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

// topSearchTerms reads the most searched publishable terms, and reports how
// many candidates the text policy held back.
//
// public_scope is required in SQL, so a family-scoped search is never even
// read out of the table; PublicQueryText then decides on the text.
func (r *HomepageRepository) topSearchTerms(ctx context.Context, window RoomStatsWindow, limit int, monitored []string) ([]SearchTerm, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT query_normalized,
		       COUNT(*) AS searches,
		       COUNT(*) FILTER (WHERE results_count > 0) AS with_results
		  FROM search_queries
		 WHERE searched_at >= NOW() - $1::interval
		   AND public_scope
		   AND `+notMonitoredExpr+`
		 GROUP BY query_normalized
		HAVING COUNT(*) >= $3
		 ORDER BY searches DESC, query_normalized
		 LIMIT $4
	`, window.interval(), monitored, searchTermMinOccurrences, limit+searchTermOverfetch)
	if err != nil {
		LogQueryError(ctx, "topSearchTerms", "search_queries", err)
		return nil, 0, fmt.Errorf("top search terms: %w", err)
	}
	defer rows.Close()

	published := true
	terms := make([]SearchTerm, 0, limit)
	withheld := 0
	for rows.Next() {
		var term SearchTerm
		if err := rows.Scan(&term.Query, &term.Count, &term.WithResults); err != nil {
			return nil, 0, fmt.Errorf("scan top search term: %w", err)
		}
		if PublicQueryText(term.Query, &published) != PublicQueryOK {
			withheld++
			continue
		}
		if len(terms) < limit {
			terms = append(terms, term)
		}
	}
	return terms, withheld, rows.Err()
}

// recentSearchTerms reads the most recently searched publishable terms, each
// at its latest search, with the searcher type that request recorded. Terms
// are aggregated first and only the chosen terms' latest searches are read
// back, instead of ranking every search in the window.
func (r *HomepageRepository) recentSearchTerms(ctx context.Context, window RoomStatsWindow, limit int, monitored []string) ([]RecentSearchTerm, int, error) {
	rows, err := r.pool.Query(ctx, `
		WITH candidates AS (
		  SELECT query_normalized, COUNT(*) AS occurrences, MAX(searched_at) AS last_searched
		    FROM search_queries
		   WHERE searched_at >= NOW() - $1::interval
		     AND public_scope
		     AND `+notMonitoredExpr+`
		   GROUP BY query_normalized
		  HAVING COUNT(*) >= $3
		   ORDER BY last_searched DESC, query_normalized
		   LIMIT $4
		)
		SELECT c.query_normalized, latest.searcher_type, latest.results_count, c.last_searched, c.occurrences
		  FROM candidates c
		 CROSS JOIN LATERAL (
		   SELECT searcher_type, results_count
		     FROM search_queries
		    WHERE query_normalized = c.query_normalized
		      AND searched_at = c.last_searched
		      AND public_scope
		      AND `+notMonitoredExpr+`
		    LIMIT 1
		 ) latest
		 ORDER BY c.last_searched DESC, c.query_normalized
	`, window.interval(), monitored, searchTermMinOccurrences, limit+searchTermOverfetch)
	if err != nil {
		LogQueryError(ctx, "recentSearchTerms", "search_queries", err)
		return nil, 0, fmt.Errorf("recent search terms: %w", err)
	}
	defer rows.Close()

	published := true
	terms := make([]RecentSearchTerm, 0, limit)
	withheld := 0
	for rows.Next() {
		var term RecentSearchTerm
		if err := rows.Scan(&term.Query, &term.SearcherType, &term.ResultsCount,
			&term.LastSearched, &term.Occurrences); err != nil {
			return nil, 0, fmt.Errorf("scan recent search term: %w", err)
		}
		if PublicQueryText(term.Query, &published) != PublicQueryOK {
			withheld++
			continue
		}
		if len(terms) < limit {
			terms = append(terms, term)
		}
	}
	return terms, withheld, rows.Err()
}

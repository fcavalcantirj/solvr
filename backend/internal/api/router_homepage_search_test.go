package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /v1/homepage/search, end to end against the real router and the real
// database, with NO credentials at all.
//
// The whole point of this endpoint is that it publishes search ACTIVITY
// without publishing the search LOG, so most of what is asserted here is about
// what does NOT come out of it: a family-scoped search's term, a term nobody
// repeated, a credential somebody pasted into the box, an email address, a
// monitor's traffic, and an anonymous caller reported as a person.

type hpoSearchRow struct {
	Query            string `json:"query"`
	Count            int    `json:"count"`
	CountLabel       string `json:"count_label"`
	WithResults      int    `json:"with_results"`
	WithResultsLabel string `json:"with_results_label"`
	SearchURL        string `json:"search_url"`
	SearchLabel      string `json:"search_label"`
}

type hpoRecentSearchRow struct {
	Query         string `json:"query"`
	SearcherLabel string `json:"searcher_label"`
	TimeLabel     string `json:"time_label"`
	CountLabel    string `json:"count_label"`
	ResultsLabel  string `json:"results_label"`
	SearchURL     string `json:"search_url"`
}

type hpoSearch struct {
	Heading        string `json:"heading"`
	Intro          string `json:"intro"`
	WindowLabel    string `json:"window_label"`
	SelectedWindow string `json:"selected_window"`
	WindowOptions  []struct {
		Value    string `json:"value"`
		Label    string `json:"label"`
		Selected bool   `json:"selected"`
	} `json:"window_options"`
	Metrics    []hpoMetric `json:"metrics"`
	Monitoring *hpoMetric  `json:"monitoring"`
	Series     struct {
		Sparkline *struct {
			Label  string `json:"label"`
			Window string `json:"window"`
			Points []struct {
				Label  string `json:"label"`
				Value  int    `json:"value"`
				Height string `json:"height"`
			} `json:"points"`
		} `json:"sparkline"`
		TableHeading string `json:"table_heading"`
		TableCaption string `json:"table_caption"`
		PeriodHeader string `json:"period_header"`
		CountHeader  string `json:"count_header"`
		Rows         []struct {
			Label   string `json:"label"`
			Value   int    `json:"value"`
			Display string `json:"display"`
		} `json:"rows"`
	} `json:"series"`
	Top struct {
		Heading           string         `json:"heading"`
		Window            string         `json:"window"`
		Definition        string         `json:"definition"`
		QueryHeader       string         `json:"query_header"`
		CountHeader       string         `json:"count_header"`
		WithResultsHeader string         `json:"with_results_header"`
		Rows              []hpoSearchRow `json:"rows"`
		EmptyNote         string         `json:"empty_note"`
		WithheldNote      string         `json:"withheld_note"`
	} `json:"top"`
	Recent struct {
		Heading    string               `json:"heading"`
		Window     string               `json:"window"`
		Definition string               `json:"definition"`
		Rows       []hpoRecentSearchRow `json:"rows"`
		EmptyNote  string               `json:"empty_note"`
	} `json:"recent"`
	PrivacyNote string `json:"privacy_note"`
}

func (s hpoSearch) metric(key string) hpoMetric {
	for _, m := range s.Metrics {
		if m.Key == key {
			return m
		}
	}
	return hpoMetric{}
}

func (s hpoSearch) topTerms() []string {
	out := make([]string, 0, len(s.Top.Rows))
	for _, row := range s.Top.Rows {
		out = append(out, row.Query)
	}
	return out
}

func (s hpoSearch) recentTerms() []string {
	out := make([]string, 0, len(s.Recent.Rows))
	for _, row := range s.Recent.Rows {
		out = append(out, row.Query)
	}
	return out
}

// getHomepageSearch calls the endpoint with no credentials at all.
func getHomepageSearch(t *testing.T, baseURL, window string) (hpoSearch, string) {
	t.Helper()

	url := baseURL + "/v1/homepage/search"
	if window != "" {
		url += "?window=" + window
	}
	resp, err := http.Get(url) //nolint:gosec,noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", body)

	var envelope struct {
		Data hpoSearch `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Data, string(body)
}

func TestHomepageSearch_ServesTheFourTotalsWithoutCredentials(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoInsertSearchesAs(t, pool, "hpo agent term", 3, "agent", true, "")
	hpoInsertSearchesAs(t, pool, "hpo human term", 2, "human", true, "")
	hpoInsertSearchesAs(t, pool, "hpo anonymous term", 2, "anonymous", true, "")

	section, _ := getHomepageSearch(t, ts.URL, "")

	require.Len(t, section.Metrics, 4)
	for _, m := range section.Metrics {
		assert.NotEmpty(t, m.Window, "metric %q window", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
		assert.NotEmpty(t, m.Display, "metric %q display", m.Key)
	}

	total := section.metric("searches")
	agent := section.metric("agent_searches")
	human := section.metric("human_searches")
	anon := section.metric("anonymous_searches")

	assert.GreaterOrEqual(t, agent.Value, 3)
	assert.GreaterOrEqual(t, human.Value, 2)
	assert.GreaterOrEqual(t, anon.Value, 2)
	assert.Equal(t, total.Value, agent.Value+human.Value+anon.Value,
		"the three breakdowns account for every counted search")

	assert.Equal(t, "24h", section.SelectedWindow)
	require.Len(t, section.WindowOptions, 3)
	assert.NotEmpty(t, section.PrivacyNote)
}

func TestHomepageSearch_WindowSelectorReReadsTheSection(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoInsertSearches(t, pool, "hpo windowed term", 2)

	for _, window := range []string{"24h", "7d", "30d"} {
		section, _ := getHomepageSearch(t, ts.URL, window)
		assert.Equal(t, window, section.SelectedWindow, "window %s", window)

		selected := 0
		for _, option := range section.WindowOptions {
			if option.Selected {
				selected++
				assert.Equal(t, window, option.Value)
			}
		}
		assert.Equal(t, 1, selected, "exactly one window is selected")

		for _, m := range section.Metrics {
			assert.Contains(t, m.Window, "last", "metric %q states its window", m.Key)
		}
		require.NotNil(t, section.Series.Sparkline, "window %s", window)
		assert.Len(t, section.Series.Rows, len(section.Series.Sparkline.Points),
			"the table carries every point the chart draws")
	}

	// An unrecognised window falls back to the default, and the response says
	// which window it actually measured, so it cannot mislabel an answer.
	fallback, _ := getHomepageSearch(t, ts.URL, "all-time")
	assert.Equal(t, "24h", fallback.SelectedWindow)
}

func TestHomepageSearch_NeverPublishesAProtectedOrUnrepeatedTerm(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	familyTerm := "hpo what the family private post says"
	onceTerm := "hpo my private medical question"
	publicTerm := "hpo postgres connection pool exhausted"

	// A signed-in human's search runs family-scoped: it could have matched a
	// post that is not public, so the term that found it is not a public term.
	hpoInsertSearchesAs(t, pool, familyTerm, 4, "human", false, "")
	hpoInsertSearchesAs(t, pool, onceTerm, 1, "agent", true, "")
	hpoInsertSearchesAs(t, pool, publicTerm, 3, "agent", true, "")

	section, raw := getHomepageSearch(t, ts.URL, "")

	assert.NotContains(t, raw, familyTerm,
		"a family-scoped search term never reaches a public page")
	assert.NotContains(t, raw, onceTerm,
		"one search belongs to one searcher")
	assert.Contains(t, section.topTerms(), publicTerm)
	assert.Contains(t, section.recentTerms(), publicTerm)

	// The family searches are still COUNTED — withheld text is not withheld
	// activity.
	assert.GreaterOrEqual(t, section.metric("human_searches").Value, 4)
}

func TestHomepageSearch_NeverPublishesPastedSecretsOrPersonalData(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	secret := "hpo solvr_sk_9f2c1ab4d77e4f0a91bbcd23"
	email := "hpo why does someone@example.com bounce"
	safe := "hpo goroutine leak on shutdown"

	hpoInsertSearches(t, pool, secret, 3)
	hpoInsertSearches(t, pool, email, 3)
	hpoInsertSearches(t, pool, safe, 3)

	section, raw := getHomepageSearch(t, ts.URL, "")

	assert.NotContains(t, raw, "solvr_sk_9f2c1ab4d77e4f0a91bbcd23")
	assert.NotContains(t, raw, "someone@example.com")
	assert.Contains(t, section.topTerms(), safe)

	// And the page says it held something back rather than quietly shrinking.
	assert.Contains(t, section.Top.WithheldNote, "withheld")
}

func TestHomepageSearch_SeparatesKnownMonitoringFromTheDefaultView(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	before, _ := getHomepageSearch(t, ts.URL, "")

	monitorTerm := "hpo monitor probe term"
	hpoInsertSearchesAs(t, pool, monitorTerm, 5, "anonymous", true, "UptimeRobot/2.0; http://uptimerobot.com/")

	after, raw := getHomepageSearch(t, ts.URL, "")

	assert.Equal(t, before.metric("searches").Value, after.metric("searches").Value,
		"monitoring is not in the headline total")
	assert.Equal(t, before.metric("anonymous_searches").Value, after.metric("anonymous_searches").Value,
		"monitoring is not in the anonymous figure either")

	require.NotNil(t, after.Monitoring)
	assert.Equal(t, 5, after.Monitoring.Value-before.Monitoring.Value)
	assert.NotEmpty(t, after.Monitoring.Definition)

	assert.NotContains(t, raw, monitorTerm, "a monitored term is not a search term")
}

func TestHomepageSearch_EveryPublishedTermLinksToCanonicalPostsSearch(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	term := "hpo sse reconnect cursor gap"
	hpoInsertSearches(t, pool, term, 3)

	section, _ := getHomepageSearch(t, ts.URL, "")

	rows := 0
	for _, row := range section.Top.Rows {
		rows++
		assert.True(t, strings.HasPrefix(row.SearchURL, "/posts?q="),
			"row %q must open canonical Posts search, got %q", row.Query, row.SearchURL)
		assert.NotContains(t, row.SearchURL, "type=", "no legacy problem/idea filter")
		assert.NotContains(t, row.SearchURL, "token", "nothing that requires an account")
		assert.NotEmpty(t, row.SearchLabel)
		assert.NotEmpty(t, row.CountLabel)
		assert.NotEmpty(t, row.WithResultsLabel)
		assert.LessOrEqual(t, row.WithResults, row.Count)
	}
	require.Positive(t, rows)

	for _, row := range section.Recent.Rows {
		assert.True(t, strings.HasPrefix(row.SearchURL, "/posts?q="))
		assert.NotContains(t, row.SearchURL, "type=")
		assert.Contains(t, []string{"Agent", "Human", "Anonymous"}, row.SearcherLabel)
	}
}

func TestHomepageSearch_ReportsAnonymousAsAnonymous(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	term := "hpo anonymous visitor term"
	hpoInsertSearchesAs(t, pool, term, 3, "anonymous", true, "")

	section, _ := getHomepageSearch(t, ts.URL, "")

	found := false
	for _, row := range section.Recent.Rows {
		if row.Query == term {
			found = true
			assert.Equal(t, "Anonymous", row.SearcherLabel,
				"an unauthenticated search is never reported as a human")
		}
	}
	assert.True(t, found, "the term should be published: %v", section.recentTerms())
}

func TestHomepageSearch_CountsALegacyRowAndNeverQuotesIt(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	legacy := "hpo recorded before scope existed"
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(t.Context(),
			`INSERT INTO search_queries
			   (query, query_normalized, results_count, search_method, duration_ms,
			    searcher_type, searched_at, public_scope)
			 VALUES ($1, $1, 2, 'hybrid', 12, 'anonymous', NOW(), NULL)`, legacy)
		require.NoError(t, err)
	}

	section, raw := getHomepageSearch(t, ts.URL, "")

	assert.NotContains(t, raw, legacy,
		"a search of unknown eligibility contributes to counts, never to excerpts")
	assert.GreaterOrEqual(t, section.metric("anonymous_searches").Value, 3,
		"it is still counted")
	assert.Contains(t, section.metric("searches").Qualifier, "never published",
		"and the page labels that aggregate as count-only")
}

func TestHomepageSearch_ChartAddsUpToTheTotalBesideIt(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoInsertSearches(t, pool, "hpo chart term", 4)

	for _, window := range []string{"24h", "7d", "30d"} {
		section, _ := getHomepageSearch(t, ts.URL, window)

		require.NotNil(t, section.Series.Sparkline, "window %s", window)
		assert.NotEmpty(t, section.Series.TableHeading)
		assert.NotEmpty(t, section.Series.TableCaption)
		assert.NotEmpty(t, section.Series.PeriodHeader)
		assert.NotEmpty(t, section.Series.CountHeader)

		total := 0
		for i, row := range section.Series.Rows {
			total += row.Value
			assert.Equal(t, section.Series.Sparkline.Points[i].Label, row.Label)
			assert.Equal(t, section.Series.Sparkline.Points[i].Value, row.Value)
		}
		assert.Equal(t, section.metric("searches").Value, total,
			"window %s: the chart and the total are one measurement", window)
	}
}

func TestHomepageSearch_IsCacheableAndNeedsNoAuth(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()

	resp, err := http.Get(ts.URL + "/v1/homepage/search") //nolint:gosec,noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "public, max-age=30", resp.Header.Get("Cache-Control"))
}

// TestHomepageSearch_RecordsTheScopeOfARealSearch proves the instrumentation
// end to end: an unauthenticated GET /v1/search must be stored as a
// public-scope search, or nothing could ever be published.
func TestHomepageSearch_RecordsTheScopeOfARealSearch(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	// A letter suffix, not a timestamp: an 18-digit run is exactly what the
	// personal-data rule withholds, and this probe is about scope.
	term := "hpo scope probe " + strings.ToLower(time.Now().Format("jan021504050"))
	for i := 0; i < 2; i++ {
		resp, err := http.Get(ts.URL + "/v1/search?q=" + strings.ReplaceAll(term, " ", "+")) //nolint:gosec,noctx
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// The analytics insert is fire-and-forget, so wait for the row rather than
	// assume it landed.
	var scope *bool
	var searcherType string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		err := pool.QueryRow(t.Context(),
			`SELECT public_scope, searcher_type FROM search_queries
			  WHERE query_normalized = $1 ORDER BY id DESC LIMIT 1`, term).
			Scan(&scope, &searcherType)
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	require.NotNil(t, scope, "an unauthenticated search must record its scope")
	assert.True(t, *scope, "an unauthenticated search runs over public content only")
	assert.Equal(t, "anonymous", searcherType)

	// db.PublicQueryText is what the page consults; prove the recorded value
	// satisfies it rather than asserting the column in isolation.
	assert.Equal(t, db.PublicQueryOK, db.PublicQueryText(term, scope))
}

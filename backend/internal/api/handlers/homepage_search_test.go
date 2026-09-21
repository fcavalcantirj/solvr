package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The homepage search section. Pure builders: no database, nothing skipped.
//
// What is asserted here is the WORDING and the CONTRACT — that every number
// states the window it was measured over and what it counts, that anonymous is
// its own answer rather than a stand-in for human, that monitoring is stated
// apart from the view rather than folded into it, that the chart and its
// tabular equivalent carry the same data, and that a published term links to
// canonical Posts search with no legacy type filter.

func searchPulseFixture() db.SearchPulse {
	window, _ := db.RoomStatsWindowByValue("7d")
	now := time.Now()
	// The bucket counts add up to Eligible below: a chart that did not add up
	// to the total printed beside it would be two different measurements.
	counts := []int{10, 15, 20, 25, 20, 18, 12}
	series := make([]db.BucketCount, 0, window.Buckets)
	for i, count := range counts {
		series = append(series, db.BucketCount{
			BucketStart: now.Add(-time.Duration(window.Buckets-1-i) * 24 * time.Hour),
			Count:       count,
		})
	}
	return db.SearchPulse{
		Window:       window,
		Eligible:     120,
		Agent:        80,
		Human:        25,
		Anonymous:    15,
		Monitoring:   9,
		UnknownScope: 40,
		Series:       series,
		Top: []db.SearchTerm{
			{Query: "postgres connection pool", Count: 12, WithResults: 9},
			{Query: "vitest mock hoisting", Count: 4, WithResults: 0},
		},
		Recent: []db.RecentSearchTerm{
			{Query: "postgres connection pool", SearcherType: "agent", Occurrences: 12, ResultsCount: 3, LastSearched: now.Add(-90 * time.Second)},
			{Query: "rate limit 429", SearcherType: "anonymous", Occurrences: 2, ResultsCount: 0, LastSearched: now.Add(-3 * time.Hour)},
		},
	}
}

func TestBuildOverviewSearch_ShowsTheFourEligibleTotals(t *testing.T) {
	section := buildOverviewSearch(searchPulseFixture())

	byKey := map[string]OverviewMetric{}
	for _, m := range section.Metrics {
		byKey[m.Key] = m
		assert.NotEmpty(t, m.Window, "metric %q must state its window", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q must define what it counts", m.Key)
		assert.NotEmpty(t, m.Display, "metric %q must arrive formatted", m.Key)
	}
	require.Len(t, section.Metrics, 4, "searches, agent, human, anonymous")

	assert.Equal(t, 120, byKey["searches"].Value)
	assert.Equal(t, 80, byKey["agent_searches"].Value)
	assert.Equal(t, 25, byKey["human_searches"].Value)
	assert.Equal(t, 15, byKey["anonymous_searches"].Value)

	for _, m := range section.Metrics {
		assert.Equal(t, "last 7 days", m.Window, "metric %q follows the selected window", m.Key)
	}
	assert.Equal(t, "7d", section.SelectedWindow)
	require.Len(t, section.WindowOptions, 3)
	for _, o := range section.WindowOptions {
		assert.Equal(t, o.Value == "7d", o.Selected)
	}
}

func TestBuildOverviewSearch_AnonymousIsNotDescribedAsHuman(t *testing.T) {
	section := buildOverviewSearch(searchPulseFixture())

	byKey := map[string]OverviewMetric{}
	for _, m := range section.Metrics {
		byKey[m.Key] = m
	}

	anon := byKey["anonymous_searches"]
	assert.Contains(t, strings.ToLower(anon.Label), "anonymous")
	assert.Contains(t, strings.ToLower(anon.Definition), "not assumed",
		"the definition must say the traffic is not assumed to be human")

	human := byKey["human_searches"]
	assert.Contains(t, strings.ToLower(human.Definition), "signed-in",
		"a human search is one the request authenticated as a person")
}

func TestBuildOverviewSearch_StatesMonitoringApartFromTheView(t *testing.T) {
	section := buildOverviewSearch(searchPulseFixture())

	require.NotNil(t, section.Monitoring)
	assert.Equal(t, 9, section.Monitoring.Value)
	assert.Contains(t, strings.ToLower(section.Monitoring.Definition), "excluded",
		"the separated figure must say it is out of the numbers above")

	for _, m := range section.Metrics {
		assert.NotEqual(t, "monitoring", m.Key, "monitoring is never one of the headline totals")
	}
}

func TestBuildOverviewSearch_LabelsTheCountOnlyAggregate(t *testing.T) {
	section := buildOverviewSearch(searchPulseFixture())

	byKey := map[string]OverviewMetric{}
	for _, m := range section.Metrics {
		byKey[m.Key] = m
	}
	qualifier := strings.ToLower(byKey["searches"].Qualifier)
	assert.Contains(t, qualifier, "40", "the count-only share is stated as a number")
	assert.Contains(t, qualifier, "never published",
		"a search of unknown eligibility is counted, and the page says its text is not published")

	assert.NotEmpty(t, section.PrivacyNote)
	assert.Contains(t, strings.ToLower(section.PrivacyNote), "never publishes raw")
}

func TestBuildOverviewSearch_TopTableLinksToCanonicalPostsSearch(t *testing.T) {
	section := buildOverviewSearch(searchPulseFixture())

	require.Len(t, section.Top.Rows, 2)
	row := section.Top.Rows[0]
	assert.Equal(t, "postgres connection pool", row.Query)
	assert.Equal(t, 12, row.Count)
	assert.Equal(t, "12 searches", row.CountLabel)
	assert.Equal(t, 9, row.WithResults)
	assert.Contains(t, row.WithResultsLabel, "9")

	assert.Equal(t, "/posts?q=postgres+connection+pool", row.SearchURL,
		"an eligible query opens canonical Posts search")
	assert.NotContains(t, row.SearchURL, "type=")
	assert.NotContains(t, row.SearchURL, "problem")
	assert.NotContains(t, row.SearchURL, "idea")
	assert.NotEmpty(t, row.SearchLabel)

	assert.Equal(t, "0 of 4 found something", section.Top.Rows[1].WithResultsLabel,
		"a term that finds nothing says so rather than showing a bare zero")
	assert.NotEmpty(t, section.Top.QueryHeader)
	assert.NotEmpty(t, section.Top.CountHeader)
	assert.NotEmpty(t, section.Top.WithResultsHeader)
	assert.NotEmpty(t, section.Top.EmptyNote)
}

func TestBuildOverviewSearch_RecentListCarriesTimeAndSearcherType(t *testing.T) {
	section := buildOverviewSearch(searchPulseFixture())

	require.Len(t, section.Recent.Rows, 2)

	first := section.Recent.Rows[0]
	assert.Equal(t, "postgres connection pool", first.Query)
	assert.Equal(t, "Agent", first.SearcherLabel)
	assert.NotEmpty(t, first.TimeLabel)
	assert.Equal(t, "/posts?q=postgres+connection+pool", first.SearchURL)

	second := section.Recent.Rows[1]
	assert.Equal(t, "Anonymous", second.SearcherLabel,
		"an unauthenticated search is reported as anonymous, never as human")
	assert.Contains(t, second.ResultsLabel, "no result")

	assert.Contains(t, strings.ToLower(section.Recent.Definition), "more than once",
		"the page states the privacy floor it applies")
}

func TestBuildOverviewSearch_ChartAndTableCarryTheSameData(t *testing.T) {
	pulse := searchPulseFixture()
	section := buildOverviewSearch(pulse)

	require.NotNil(t, section.Series.Sparkline)
	assert.Len(t, section.Series.Sparkline.Points, pulse.Window.Buckets)
	require.Len(t, section.Series.Rows, pulse.Window.Buckets,
		"the accessible table must carry every point the chart draws")

	for i, point := range section.Series.Sparkline.Points {
		assert.Equal(t, point.Label, section.Series.Rows[i].Label)
		assert.Equal(t, point.Value, section.Series.Rows[i].Value)
		assert.NotEmpty(t, section.Series.Rows[i].Display)
	}

	total := 0
	for _, row := range section.Series.Rows {
		total += row.Value
	}
	assert.Equal(t, pulse.Eligible, total,
		"the chart illustrates the same eligible total stated above it")

	assert.Contains(t, section.Series.Sparkline.Window, section.Metrics[0].Window,
		"the chart states the same window as the numbers")
	assert.NotEmpty(t, section.Series.TableHeading)
	assert.NotEmpty(t, section.Series.TableCaption)
	assert.NotEmpty(t, section.Series.PeriodHeader)
	assert.NotEmpty(t, section.Series.CountHeader)
}

func TestBuildOverviewSearch_EscapesATermIntoItsSearchURL(t *testing.T) {
	pulse := searchPulseFixture()
	pulse.Top = []db.SearchTerm{{Query: "c++ & rust <interop>", Count: 2, WithResults: 1}}
	pulse.Recent = nil

	section := buildOverviewSearch(pulse)
	require.Len(t, section.Top.Rows, 1)
	assert.Equal(t, "/posts?q=c%2B%2B+%26+rust+%3Cinterop%3E", section.Top.Rows[0].SearchURL)
}

func TestBuildOverviewSearch_EmptyWindowStillExplainsItself(t *testing.T) {
	window, _ := db.RoomStatsWindowByValue("24h")
	section := buildOverviewSearch(db.SearchPulse{Window: window})

	assert.Empty(t, section.Top.Rows)
	assert.Empty(t, section.Recent.Rows)
	assert.NotEmpty(t, section.Top.EmptyNote)
	assert.NotEmpty(t, section.Recent.EmptyNote)
	assert.Nil(t, section.Series.Sparkline, "no measurement draws no chart")
	assert.Empty(t, section.Series.Rows)
	for _, m := range section.Metrics {
		assert.Equal(t, "0", m.Display)
		assert.False(t, m.Unavailable, "zero searches is a measurement, not a missing one")
	}
	assert.Empty(t, section.Metrics[0].Qualifier,
		"with nothing of unknown eligibility there is nothing to qualify")
}

func TestBuildOverviewSearch_SaysHowManyTermsItWithheld(t *testing.T) {
	pulse := searchPulseFixture()
	pulse.WithheldTerms = 3
	section := buildOverviewSearch(pulse)

	assert.Contains(t, section.Top.WithheldNote, "3")
	assert.Contains(t, strings.ToLower(section.Top.WithheldNote), "withheld")

	pulse.WithheldTerms = 0
	assert.Empty(t, buildOverviewSearch(pulse).Top.WithheldNote,
		"nothing withheld says nothing")
}

func TestSearcherTypeLabel_NamesEveryRecordedType(t *testing.T) {
	assert.Equal(t, "Agent", searcherTypeLabel("agent"))
	assert.Equal(t, "Human", searcherTypeLabel("human"))
	assert.Equal(t, "Anonymous", searcherTypeLabel("anonymous"))
	assert.Equal(t, "Anonymous", searcherTypeLabel(""),
		"an unrecorded searcher is anonymous, never human")
	assert.Equal(t, "Anonymous", searcherTypeLabel("something new"))
}

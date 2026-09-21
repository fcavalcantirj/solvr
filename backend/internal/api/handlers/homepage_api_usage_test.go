package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The API-activity section: what Solvr publishes about how much its API is
// called, and what it refuses to say with those numbers.

func usagePulse(window db.RoomStatsWindow, since time.Time) db.APIUsagePulse {
	return db.APIUsagePulse{
		Window:                  window,
		SuccessfulCalls:         4210,
		AgentCalls:              3100,
		HumanCalls:              900,
		AnonymousCalls:          210,
		PassivePolls:            2600,
		WriteAndSearchCalls:     1610,
		RoomKnowledgeOperations: 11,
		InstrumentedSince:       &since,
		Series: []db.BucketCount{
			{BucketStart: time.Now().Add(-2 * time.Hour).UTC(), Count: 40},
			{BucketStart: time.Now().Add(-time.Hour).UTC(), Count: 120},
		},
	}
}

func metricsByKey(metrics []OverviewMetric) map[string]OverviewMetric {
	byKey := make(map[string]OverviewMetric, len(metrics))
	for _, m := range metrics {
		byKey[m.Key] = m
	}
	return byKey
}

func TestOverviewAPIUsage_PublishesMeasuredCallVolumeForTheSelectedWindow(t *testing.T) {
	window := db.RoomStatsWindows[1] // 7 days
	section := buildOverviewAPIUsage(usagePulse(window, time.Now().Add(-90*24*time.Hour)))

	require.NotEmpty(t, section.Metrics)
	byKey := metricsByKey(section.Metrics)

	require.Contains(t, byKey, "api_calls_succeeded")
	require.Contains(t, byKey, "agent_api_calls")
	require.Contains(t, byKey, "room_knowledge_operations")
	assert.Equal(t, 4210, byKey["api_calls_succeeded"].Value)
	assert.Equal(t, "4,210", byKey["api_calls_succeeded"].Display)
	assert.Equal(t, 3100, byKey["agent_api_calls"].Value)
	assert.Equal(t, 11, byKey["room_knowledge_operations"].Value)

	for _, m := range section.Metrics {
		assert.Equal(t, window.WindowText, m.Window, "metric %q must state the window it was measured over", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
		assert.False(t, m.Presence, "a call volume is measured over a window, never at this moment")
	}

	assert.Equal(t, window.Value, section.SelectedWindow)
	assert.Len(t, section.WindowOptions, len(db.RoomStatsWindows))
	selected := 0
	for _, option := range section.WindowOptions {
		if option.Selected {
			selected++
			assert.Equal(t, window.Value, option.Value)
		}
	}
	assert.Equal(t, 1, selected, "exactly one window is selected")
}

// Passive polling is reported apart from create/send/search, and the two
// partition the total: a reader can see how much of the volume is waiting.
func TestOverviewAPIUsage_ShowsPassivePollingApartFromCreateSendAndSearch(t *testing.T) {
	window := db.DefaultRoomStatsWindow()
	section := buildOverviewAPIUsage(usagePulse(window, time.Now().Add(-90*24*time.Hour)))
	byKey := metricsByKey(section.Metrics)

	require.Contains(t, byKey, "passive_poll_calls")
	require.Contains(t, byKey, "write_and_search_calls")
	assert.Equal(t, 2600, byKey["passive_poll_calls"].Value)
	assert.Equal(t, 1610, byKey["write_and_search_calls"].Value)
	assert.Equal(t,
		byKey["api_calls_succeeded"].Value,
		byKey["passive_poll_calls"].Value+byKey["write_and_search_calls"].Value,
		"the two halves must account for every successful call")
}

// The numbers are call volume. They are never worded as people, and never as
// independent successful tasks.
func TestOverviewAPIUsage_ReadsAsVolumeNeverAsPeopleOrTasks(t *testing.T) {
	section := buildOverviewAPIUsage(usagePulse(db.DefaultRoomStatsWindow(), time.Now().Add(-90*24*time.Hour)))

	byKey := metricsByKey(section.Metrics)
	total := byKey["api_calls_succeeded"]
	assert.Contains(t, strings.ToLower(total.Definition+" "+total.Qualifier), "request",
		"the headline number has to say it counts requests")
	assert.Contains(t, strings.ToLower(total.Qualifier), "retr",
		"a retry the domain deduplicated is still a request that arrived, and the number must say so")

	authored := []string{section.Heading, section.Intro, section.ScopeNote}
	for _, m := range section.Metrics {
		authored = append(authored, m.Label, m.Definition, m.Qualifier)
	}
	for _, text := range authored {
		lower := strings.ToLower(text)
		for _, forbidden := range []string{"people", "users", "tasks solved", "independent successful"} {
			assert.NotContains(t, lower, forbidden,
				"a call volume may not be worded as %q: %q", forbidden, text)
		}
		assert.Empty(t, PrivateAnalyticsTermIn(text),
			"the section published an excluded idea: %q", text)
	}
}

// Aggregation across private and public operations is anonymous and
// platform-level. Nothing in the payload can be read down to a room, an
// account or a request.
func TestOverviewAPIUsage_AggregatesPrivateAndPublicAnonymously(t *testing.T) {
	section := buildOverviewAPIUsage(usagePulse(db.DefaultRoomStatsWindow(), time.Now().Add(-90*24*time.Hour)))

	assert.NotEmpty(t, section.ScopeNote, "the section must state what it aggregates")
	lower := strings.ToLower(section.ScopeNote)
	assert.Contains(t, lower, "private")
	assert.Contains(t, lower, "public")

	// The section carries counts and authored text only: no row, list or map
	// that could break a total down by room or account.
	assert.NotEmpty(t, section.Endpoints, "the endpoint list is documentation, not a breakdown")
	for _, endpoint := range section.Endpoints {
		assert.True(t, strings.HasPrefix(endpoint.Path, "/v1/"), "endpoint path %q", endpoint.Path)
		assert.NotContains(t, endpoint.Path, "{slug}", "a documented path may not carry an identifier")
	}
}

// Before instrumentation there is no measurement. A zero would be a claim
// that nothing happened, which is a different statement.
func TestOverviewAPIUsage_ReportsUnmeasuredRatherThanZero(t *testing.T) {
	window := db.DefaultRoomStatsWindow()
	section := buildOverviewAPIUsage(db.APIUsagePulse{Window: window})

	require.NotEmpty(t, section.Metrics)
	for _, m := range section.Metrics {
		assert.True(t, m.Unavailable, "metric %q must be unavailable before instrumentation", m.Key)
		assert.Equal(t, unreadMetricDisplay, m.Display, "metric %q display", m.Key)
		assert.NotEmpty(t, m.Qualifier, "metric %q must say why it is unavailable", m.Key)
	}
	assert.Nil(t, section.Series.Sparkline, "there is no series before there is a measurement")
	assert.NotEmpty(t, section.StartNote, "the section must say measurement has not started")
}

// History from before instrumentation is unavailable, never estimated. The
// figure says since when it has been measured.
func TestOverviewAPIUsage_LabelsHistoryBeforeInstrumentationUnavailable(t *testing.T) {
	window := db.RoomStatsWindows[2] // 30 days
	since := time.Now().Add(-3 * 24 * time.Hour)
	section := buildOverviewAPIUsage(usagePulse(window, since))

	byKey := metricsByKey(section.Metrics)
	total := byKey["api_calls_succeeded"]
	assert.False(t, total.Unavailable, "the measured part of the window is still a measurement")
	assert.Contains(t, total.Qualifier, since.UTC().Format("2 Jan 2006"))
	assert.Contains(t, strings.ToLower(total.Qualifier), "unavailable")

	assert.Contains(t, strings.ToLower(section.StartNote), "unavailable",
		"the section says earlier history is unavailable rather than zero")

	// A window entirely inside the instrumented period needs no such caveat.
	old := buildOverviewAPIUsage(usagePulse(db.DefaultRoomStatsWindow(), time.Now().Add(-90*24*time.Hour)))
	assert.NotEmpty(t, metricsByKey(old.Metrics)["api_calls_succeeded"].Qualifier,
		"the qualifier still states what request volume means")
	assert.NotContains(t, strings.ToLower(metricsByKey(old.Metrics)["api_calls_succeeded"].Qualifier), "unavailable")
	assert.Empty(t, old.StartNote)
}

// The chart and its table are built from one series, so they cannot disagree.
func TestOverviewAPIUsage_ChartAndTableCarryTheSameSeries(t *testing.T) {
	window := db.DefaultRoomStatsWindow()
	section := buildOverviewAPIUsage(usagePulse(window, time.Now().Add(-90*24*time.Hour)))

	require.NotNil(t, section.Series.Sparkline)
	require.Len(t, section.Series.Sparkline.Points, 2)
	require.Len(t, section.Series.Rows, 2)

	assert.Equal(t, 120, section.Series.Sparkline.MaxValue)
	assert.Equal(t, "100.0%", section.Series.Sparkline.Points[1].Height)
	assert.InDelta(t, 1.0/3.0, section.Series.Sparkline.Points[0].Normalized, 0.001,
		"the API normalises the heights so the browser does no arithmetic")

	for i, row := range section.Series.Rows {
		assert.Equal(t, section.Series.Sparkline.Points[i].Label, row.Label)
		assert.Equal(t, section.Series.Sparkline.Points[i].Value, row.Value)
		assert.Equal(t, formatOverviewNumber(row.Value), row.Display)
	}
	assert.NotEmpty(t, section.Series.TableCaption)
	assert.NotEmpty(t, section.Series.PeriodHeader)
	assert.NotEmpty(t, section.Series.CountHeader)
}

// Every metric the section publishes is one somebody named as publishable,
// filed under aggregate API usage.
func TestOverviewAPIUsage_PublishesOnlyAllowlistedMetrics(t *testing.T) {
	section := buildOverviewAPIUsage(usagePulse(db.DefaultRoomStatsWindow(), time.Now().Add(-90*24*time.Hour)))

	for _, m := range section.Metrics {
		category, allowed := PublicOverviewAllowsMetric(m.Key)
		assert.True(t, allowed, "metric %q is not on the public allowlist", m.Key)
		assert.Equal(t, CategoryAggregateAPIUsage, category, "metric %q category", m.Key)
	}
}

// A registration total sitting among call volumes would read as usage, which
// is the one thing the all-time section exists to keep separate.
func TestOverviewAPIUsage_CarriesNoRegistrationTotal(t *testing.T) {
	section := buildOverviewAPIUsage(usagePulse(db.DefaultRoomStatsWindow(), time.Now().Add(-90*24*time.Hour)))

	for _, m := range section.Metrics {
		assert.NotContains(t, strings.ToLower(m.Key), "registered",
			"a registration belongs in the all-time section, not among call volumes")
		assert.NotContains(t, strings.ToLower(m.Label), "registered")
	}
}

// Nothing in the section is derived from a message total, a post total or
// anything else that is not a recorded request.
func TestOverviewAPIUsage_IsDerivedFromRecordedRequestsAlone(t *testing.T) {
	window := db.DefaultRoomStatsWindow()
	empty := buildOverviewAPIUsage(db.APIUsagePulse{Window: window})

	for _, m := range empty.Metrics {
		assert.Zero(t, m.Value,
			"metric %q invented a value with nothing recorded to build it from", m.Key)
	}
}

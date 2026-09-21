package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The homepage's public room statistics.
//
// The contract these tests hold is honesty, not layout: presence is stated as
// "now" and is untouched by the window selector, historical metrics state the
// window they were measured over, an identity that was never authenticated is
// labelled unverified instead of being folded in silently, and a milestone
// that was never instrumented reads as unavailable rather than as zero.

func samplePulse(window db.RoomStatsWindow) db.RoomPulse {
	series := make([]db.BucketCount, window.Buckets)
	for i := range series {
		series[i] = db.BucketCount{
			BucketStart: time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour),
			Count:       i,
		}
	}

	activated := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return db.RoomPulse{
		Window: window,
		Presence: db.RoomPresenceStats{
			AgentsOnline:           5,
			VerifiedAgentsOnline:   3,
			UnverifiedAgentsOnline: 2,
			RoomsWithAgentsOnline:  2,
		},
		Stats: db.RoomWindowStats{
			RoomsWithConversation:       9,
			AgentMessages:               1234,
			HumanMessages:               56,
			UnverifiedAgentMessages:     40,
			TwoWayExchangeRooms:         4,
			ActivationInstrumentedSince: &activated,
			Series:                      series,
		},
		PublicRooms: 52,
		Messages24h: 300,
	}
}

func metricByKey(t *testing.T, metrics []OverviewMetric, key string) OverviewMetric {
	t.Helper()
	for _, m := range metrics {
		if m.Key == key {
			return m
		}
	}
	t.Fatalf("metric %q not found", key)
	return OverviewMetric{}
}

func TestOverviewRooms_ShowsTheSixStatisticsTheHomepagePromises(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.DefaultRoomStatsWindow()))

	presenceKeys := make([]string, 0, len(section.PresenceMetrics))
	for _, m := range section.PresenceMetrics {
		presenceKeys = append(presenceKeys, m.Key)
	}
	assert.Equal(t, []string{"agents_online_now", "rooms_with_agents_online_now"}, presenceKeys)

	windowedKeys := make([]string, 0, len(section.Metrics))
	for _, m := range section.Metrics {
		windowedKeys = append(windowedKeys, m.Key)
	}
	assert.Equal(t, []string{
		"rooms_with_conversation", "agent_messages", "human_messages", "rooms_with_two_way_exchanges",
	}, windowedKeys)

	for _, m := range append(append([]OverviewMetric{}, section.PresenceMetrics...), section.Metrics...) {
		assert.NotEmpty(t, m.Label, "metric %q label", m.Key)
		assert.NotEmpty(t, m.Display, "metric %q display", m.Key)
		assert.NotEmpty(t, m.Window, "metric %q must state the window it measures", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q must state what it counts", m.Key)
	}
}

func TestOverviewRooms_KeepsConversionRatesOutOfThePublicPage(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.DefaultRoomStatsWindow()))

	// The sparkline legitimately carries CSS percentages, so the check is on
	// the words the page shows, not on every byte of the payload.
	body, err := json.Marshal(struct {
		Heading   string           `json:"heading"`
		Intro     string           `json:"intro"`
		ScopeNote string           `json:"scope_note"`
		Presence  []OverviewMetric `json:"presence_metrics"`
		Metrics   []OverviewMetric `json:"metrics"`
	}{
		Heading:   section.Heading,
		Intro:     section.Intro,
		ScopeNote: section.ScopeNote,
		Presence:  section.PresenceMetrics,
		Metrics:   section.Metrics,
	})
	require.NoError(t, err)
	rendered := string(body)

	for _, forbidden := range []string{"conversion", "Conversion", "rate", "%"} {
		assert.NotContains(t, rendered, forbidden,
			"room activation conversion rates stay in internal analytics")
	}
}

func TestOverviewRooms_PresenceIsLabelledNowAndNeverFollowsTheSelector(t *testing.T) {
	for _, w := range db.RoomStatsWindows {
		section := buildOverviewRooms(samplePulse(w))

		require.NotEmpty(t, section.PresenceMetrics)
		for _, m := range section.PresenceMetrics {
			assert.True(t, m.Presence, "%q is a presence metric", m.Key)
			assert.Equal(t, "now", m.Window,
				"presence states Now whatever window %q is selected", w.Value)
		}
		assert.Equal(t, "Now", section.PresenceHeading)
	}

	day := buildOverviewRooms(samplePulse(db.RoomStatsWindows[0]))
	month := buildOverviewRooms(samplePulse(db.RoomStatsWindows[2]))
	assert.Equal(t,
		metricByKey(t, day.PresenceMetrics, "agents_online_now").Value,
		metricByKey(t, month.PresenceMetrics, "agents_online_now").Value,
		"the same presence reading survives a window change")
}

func TestOverviewRooms_HistoricalMetricsStateTheSelectedWindow(t *testing.T) {
	for _, w := range db.RoomStatsWindows {
		section := buildOverviewRooms(samplePulse(w))

		assert.Equal(t, w.Value, section.SelectedWindow)
		for _, m := range section.Metrics {
			assert.False(t, m.Presence, "%q is historical, not presence", m.Key)
			assert.Equal(t, w.WindowText, m.Window,
				"historical metric %q follows the selector", m.Key)
		}
	}
}

func TestOverviewRooms_OffersTheSharedSelectorDefaultingToTwentyFourHours(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.DefaultRoomStatsWindow()))

	require.Len(t, section.WindowOptions, 3)
	labels := make([]string, 0, 3)
	selected := 0
	for _, o := range section.WindowOptions {
		labels = append(labels, o.Label)
		assert.NotEmpty(t, o.Value)
		if o.Selected {
			selected++
			assert.Equal(t, "24h", o.Value, "24 hours is the default")
		}
	}
	assert.Equal(t, []string{"24 hours", "7 days", "30 days"}, labels)
	assert.Equal(t, 1, selected, "exactly one option is selected")
	assert.NotEmpty(t, section.WindowLabel, "the selector names itself for a screen reader")
}

func TestOverviewRooms_SelectorMovesWithTheRequestedWindow(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.RoomStatsWindows[1]))

	assert.Equal(t, "7d", section.SelectedWindow)
	for _, o := range section.WindowOptions {
		assert.Equal(t, o.Value == "7d", o.Selected, "option %q", o.Value)
	}
}

func TestOverviewRooms_NamesUnverifiedIdentitiesInsteadOfMixingThem(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.DefaultRoomStatsWindow()))

	online := metricByKey(t, section.PresenceMetrics, "agents_online_now")
	assert.Equal(t, 5, online.Value)
	require.NotEmpty(t, online.Qualifier, "the split between proven and claimed identity is stated")
	assert.Contains(t, online.Qualifier, "unverified")
	assert.Contains(t, online.Definition, "presence")

	messages := metricByKey(t, section.Metrics, "agent_messages")
	require.NotEmpty(t, messages.Qualifier)
	assert.Contains(t, messages.Qualifier, "unverified")
}

func TestOverviewRooms_SaysNothingAboutIdentityWhenThereIsNothingToSay(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	pulse.Presence = db.RoomPresenceStats{}
	pulse.Stats.AgentMessages = 0
	pulse.Stats.UnverifiedAgentMessages = 0

	section := buildOverviewRooms(pulse)

	assert.Empty(t, metricByKey(t, section.PresenceMetrics, "agents_online_now").Qualifier)
	assert.Empty(t, metricByKey(t, section.Metrics, "agent_messages").Qualifier)
}

func TestOverviewRooms_AllOnlineAgentsVerifiedSaysSo(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	pulse.Presence.VerifiedAgentsOnline = pulse.Presence.AgentsOnline
	pulse.Presence.UnverifiedAgentsOnline = 0

	online := metricByKey(t, buildOverviewRooms(pulse).PresenceMetrics, "agents_online_now")
	assert.NotContains(t, online.Qualifier, "unverified")
}

func TestOverviewRooms_UninstrumentedActivationIsUnavailableNotZero(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	pulse.Stats.ActivationInstrumentedSince = nil
	pulse.Stats.TwoWayExchangeRooms = 0

	exchanges := metricByKey(t, buildOverviewRooms(pulse).Metrics, "rooms_with_two_way_exchanges")

	assert.True(t, exchanges.Unavailable, "a milestone that was never recorded is unavailable")
	assert.Equal(t, unreadMetricDisplay, exchanges.Display, "it must never read as 0")
	assert.NotEqual(t, "0", exchanges.Display)
	assert.NotEmpty(t, exchanges.Qualifier, "the page says why the number is missing")
}

func TestOverviewRooms_ActivationOlderThanTheWindowReadsNormally(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	long := time.Now().Add(-90 * 24 * time.Hour)
	pulse.Stats.ActivationInstrumentedSince = &long

	exchanges := metricByKey(t, buildOverviewRooms(pulse).Metrics, "rooms_with_two_way_exchanges")

	assert.False(t, exchanges.Unavailable)
	assert.Equal(t, "4", exchanges.Display)
	assert.Empty(t, exchanges.Qualifier,
		"the whole window is instrumented, so there is no gap to declare")
}

func TestOverviewRooms_ActivationInsideTheWindowDeclaresThePartialHistory(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	recent := time.Now().Add(-2 * time.Hour)
	pulse.Stats.ActivationInstrumentedSince = &recent

	exchanges := metricByKey(t, buildOverviewRooms(pulse).Metrics, "rooms_with_two_way_exchanges")

	assert.False(t, exchanges.Unavailable, "what was measured is still worth showing")
	assert.Contains(t, exchanges.Qualifier, "unavailable",
		"history before instrumentation is declared unavailable, not counted as zero")
}

func TestOverviewRooms_ScopesEveryNumberToPublicRooms(t *testing.T) {
	section := buildOverviewRooms(samplePulse(db.DefaultRoomStatsWindow()))

	assert.Equal(t, "Public room activity", section.ScopeLabel)
	assert.Contains(t, section.ScopeNote, "52", "the public room count is stated as context")
	assert.Contains(t, section.ScopeNote, "Private")
}

func TestOverviewRooms_SparklineFollowsTheSelectedWindow(t *testing.T) {
	for _, w := range db.RoomStatsWindows {
		section := buildOverviewRooms(samplePulse(w))

		require.NotNil(t, section.Sparkline, "window %q", w.Value)
		assert.Len(t, section.Sparkline.Points, w.Buckets, "window %q buckets", w.Value)
		assert.Contains(t, section.Sparkline.Window, w.Label, "window %q label", w.Value)
	}
}

func TestOverviewRooms_SparklineArrivesNormalised(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	pulse.Stats.Series = []db.BucketCount{
		{BucketStart: time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC), Count: 0},
		{BucketStart: time.Date(2026, 9, 21, 11, 0, 0, 0, time.UTC), Count: 5},
		{BucketStart: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC), Count: 10},
	}

	spark := buildOverviewRooms(pulse).Sparkline

	require.NotNil(t, spark)
	assert.Equal(t, 10, spark.MaxValue)
	assert.Equal(t, []string{"0.0%", "50.0%", "100.0%"},
		[]string{spark.Points[0].Height, spark.Points[1].Height, spark.Points[2].Height})
}

func TestOverviewRooms_SparklineIsOmittedWhenThereIsNoSeries(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	pulse.Stats.Series = nil

	assert.Nil(t, buildOverviewRooms(pulse).Sparkline)
}

func TestParseRoomStatsWindow_DefaultsToTwentyFourHours(t *testing.T) {
	assert.Equal(t, "24h",
		parseRoomStatsWindow(httptest.NewRequest("GET", "/v1/homepage/rooms", nil)).Value)
}

func TestParseRoomStatsWindow_AcceptsEveryOfferedWindow(t *testing.T) {
	for _, w := range db.RoomStatsWindows {
		r := httptest.NewRequest("GET", "/v1/homepage/rooms?window="+w.Value, nil)
		assert.Equal(t, w.Value, parseRoomStatsWindow(r).Value)
	}
}

func TestParseRoomStatsWindow_FallsBackWithoutMislabellingTheAnswer(t *testing.T) {
	r := httptest.NewRequest("GET", "/v1/homepage/rooms?window=90d", nil)
	window := parseRoomStatsWindow(r)

	assert.Equal(t, "24h", window.Value)
	// The answer is never mislabelled: the section it builds states 24 hours.
	assert.Equal(t, "last 24 hours", buildOverviewRooms(samplePulse(window)).Metrics[0].Window)
}

func TestOverviewRooms_FlatSparklineDoesNotDivideByZero(t *testing.T) {
	pulse := samplePulse(db.DefaultRoomStatsWindow())
	pulse.Stats.Series = []db.BucketCount{
		{BucketStart: time.Date(2026, 9, 21, 1, 0, 0, 0, time.UTC), Count: 0},
		{BucketStart: time.Date(2026, 9, 21, 2, 0, 0, 0, time.UTC), Count: 0},
	}

	spark := buildOverviewRooms(pulse).Sparkline

	require.NotNil(t, spark)
	assert.Equal(t, 0, spark.MaxValue)
	for _, p := range spark.Points {
		assert.InDelta(t, 0.0, p.Normalized, 1e-9)
		assert.Equal(t, "0.0%", p.Height)
		assert.NotEmpty(t, p.Label, "each point is labelled by the API")
	}
}

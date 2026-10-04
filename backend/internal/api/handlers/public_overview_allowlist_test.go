package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Solvr measures two different things, and only one of them is public.
//
// PRODUCT ACTIVITY is what agents and people DO inside Solvr: rooms, messages,
// searches, posts, API calls. That is the product working in the open, and the
// homepage publishes it.
//
// WEBSITE TRAFFIC AND GROWTH INTELLIGENCE is what an audience does TO the
// website: visitors, sessions, page views, where they came from, what share of
// them converted and how many came back. That is operator analytics. It stays
// private, and it does not become public by being computed server-side and
// hidden in the UI — it must never leave the API at all.
//
// These tests hold that line on the serialized payload.

// buildPublicOverviewFixture builds a complete overview exactly the way
// GetOverview does, so the allowlist is tested against the real payload rather
// than against a hand-written copy of it.
func buildPublicOverviewFixture() HomepageOverview {
	window := db.DefaultRoomStatsWindow()
	pulse := db.RoomPulse{
		Window:      window,
		AllRooms:    57,
		PublicRooms: 52,
		Messages24h: 321,
		Presence:    db.RoomPresenceStats{AgentsOnline: 5, RoomsWithAgentsOnline: 2},
		// ActiveRooms24h feeds the hero's live "active rooms" slot.
		ActiveRooms24h: 4,
	}
	searchPulse := db.SearchPulse{
		Window: window, Eligible: 140, Agent: 90, Human: 30, Anonymous: 20, Monitoring: 8,
		Eligible24h: 140,
	}
	instrumentedSince := time.Now().Add(-90 * 24 * time.Hour)
	totals := &db.AllTimeTotals{AllRooms: 219, PublicRooms: 214, PublishedPosts: 2098, RegisteredAgents: 64, RegisteredHumans: 1003, RoomMessages: 8800}

	return HomepageOverview{
		HeroNumbers: buildOverviewHeroNumbers(totals, pulse, true, searchPulse, true),
		Rooms:       buildOverviewRooms(pulse, nil),
		Activity:    buildOverviewActivity(nil, overviewActivityDefaultLimit, 0, 0, time.Now()),
		Previews: buildOverviewPreviews([]db.PreviewSource{{
			Room:             models.Room{Slug: "featured-room", DisplayName: "Featured room", MessageCount: 4, LastActiveAt: time.Now()},
			Participants:     []db.RoomParticipant{{Name: "planner", AuthorType: "agent", MessageCount: 2}},
			ParticipantCount: 2,
			Ask:              &models.Message{ID: 1, AuthorType: "agent", AgentName: "planner", Content: "Build the parser", SequenceNum: intPtr(1)},
			Outcome:          &models.Message{ID: 4, AuthorType: "agent", AgentName: "executor", Content: "Parser shipped", SequenceNum: intPtr(4)},
		}}),
		APIUsage: buildOverviewAPIUsage(db.APIUsagePulse{
			Window: window, SuccessfulCalls: 4210, AgentCalls: 3100, HumanCalls: 900,
			AnonymousCalls: 210, PassivePolls: 2600, WriteAndSearchCalls: 1610,
			RoomKnowledgeOperations: 11, InstrumentedSince: &instrumentedSince,
		}),
		Search: buildOverviewSearch(searchPulse),
		Community: buildOverviewCommunity(
			totals,
			&db.AllStatsResult{TotalContributions: 3327, CrystallizedPosts: 12},
		),
		Posts:       buildOverviewPosts(nil),
		Closing:     buildOverviewClosing(),
		GeneratedAt: time.Now().UTC(),
	}
}

// Every number the page publishes has to be a number the allowlist names. A
// metric that nobody added to the allowlist is a metric nobody decided to
// publish.
func TestPublicOverviewAllowlist_NamesEveryMetricThePagePublishes(t *testing.T) {
	overview := buildPublicOverviewFixture()

	published := 0
	for section, metrics := range overviewMetricSlices(&overview) {
		for _, m := range *metrics {
			published++
			category, ok := PublicOverviewAllowsMetric(m.Key)
			assert.True(t, ok,
				"%s publishes metric %q, which is not on the public overview allowlist", section, m.Key)
			assert.NotEmpty(t, category, "metric %q must say which public category it belongs to", m.Key)
		}
	}
	require.NotNil(t, overview.Search.Monitoring)
	_, ok := PublicOverviewAllowsMetric(overview.Search.Monitoring.Key)
	assert.True(t, ok, "the separated monitoring figure is published too, so it must be allowlisted")
	published++

	assert.Equal(t, published, len(PublicOverviewMetrics),
		"the allowlist must name exactly what the page publishes — no dead entry may sit in it authorising a metric nobody shows")
}

// Every category in the allowlist is one of the five kinds of thing Solvr
// agreed to publish about itself.
func TestPublicOverviewAllowlist_UsesOnlyThePublishableCategories(t *testing.T) {
	allowed := map[PublicOverviewCategory]bool{
		CategoryRoomActivity:           true,
		CategoryRoomParticipation:      true,
		CategoryEligibleSearchActivity: true,
		CategoryAggregateAPIUsage:      true,
		CategoryProductTotals:          true,
	}

	for key, category := range PublicOverviewMetrics {
		assert.True(t, allowed[category], "metric %q is filed under %q, which is not a public category", key, category)
	}
}

// The allowlist is enforced on the way out, not in the browser. A metric the
// API was never allowed to publish is withheld from the response, whichever
// section grew it.
func TestEnforcePublicOverviewMetrics_WithholdsAnAudienceMetricFromEverySection(t *testing.T) {
	overview := buildPublicOverviewFixture()

	intruder := OverviewMetric{
		Key: "weekly_active_visitors", Label: "WEEKLY ACTIVE VISITORS", Value: 9182,
		Display: "9,182", Window: "last 7 days",
		Definition: "Distinct browsers that opened solvr.dev in the last seven days.",
	}
	for _, metrics := range overviewMetricSlices(&overview) {
		*metrics = append(*metrics, intruder)
	}
	monitoringIntruder := intruder
	overview.Search.Monitoring = &monitoringIntruder

	enforcePublicOverviewMetrics(&overview)

	for section, metrics := range overviewMetricSlices(&overview) {
		require.NotEmpty(t, *metrics, "%s: withholding one metric must not empty the section", section)
		for _, m := range *metrics {
			assert.NotEqual(t, intruder.Key, m.Key, "%s still publishes %q", section, intruder.Key)
			_, ok := PublicOverviewAllowsMetric(m.Key)
			assert.True(t, ok, "%s kept metric %q, which is not allowlisted", section, m.Key)
		}
	}
	assert.Nil(t, overview.Search.Monitoring,
		"a monitoring figure that is not the allowlisted one must be withheld, not published")

	// And the legitimate figures are all still there.
	byKey := map[string]bool{}
	for _, metrics := range overviewMetricSlices(&overview) {
		for _, m := range *metrics {
			byKey[m.Key] = true
		}
	}
	for _, key := range []string{"agents_online_now", "agent_messages", "searches", "api_calls_succeeded", "registered_humans"} {
		assert.True(t, byKey[key], "enforcing the allowlist must not drop %q", key)
	}
}

// The detector names the concept it found, so a failure says what leaked
// rather than only that something did.
func TestPrivateAnalyticsTermIn_NamesTheExcludedConcept(t *testing.T) {
	excluded := []struct{ text, concept string }{
		{"12,004 unique visitors this week", "visitors"},
		{"Sessions per visit", "sessions"},
		{"Page views over the last 30 days", "page views"},
		{"DAU / MAU ratio", "daily, weekly or monthly active users"},
		{"Monthly active users", "daily, weekly or monthly active users"},
		{"Bounce rate", "bounce"},
		{"Top acquisition channels", "acquisition"},
		{"Referrers sending the most traffic", "referrers"},
		{"Visits by country", "geography"},
		{"Signup conversion funnel", "conversion"},
		{"Week 4 retention", "retention"},
		{"Launch campaign results", "campaign"},
		{"Growth target for Q4", "growth target"},
		{"Search Console impressions", "impressions"},
	}
	for _, c := range excluded {
		assert.Equal(t, c.concept, PrivateAnalyticsTermIn(c.text), "text: %q", c.text)
	}

	// Product wording that merely looks similar is published freely.
	for _, ok := range []string{
		"Rooms with a two-way exchange",
		"A conversation between two agents",
		"Searches by agents",
		"Anyone can read this room; it can appear in public lists and search engines",
		"Agents that have registered a Solvr key, excluding deleted and suspended agents.",
		"Messages the room API accepted and delivered in public rooms.",
	} {
		assert.Empty(t, PrivateAnalyticsTermIn(ok), "text: %q", ok)
	}
}

// A JSON key is API-owned, so the same vocabulary applies to field names, and
// it applies to the whole payload rather than to text alone.
func TestPrivateAnalyticsTermInKey_ReadsFieldNames(t *testing.T) {
	for key, concept := range map[string]string{
		"unique_visitors":      "visitors",
		"sessions":             "sessions",
		"page_views":           "page views",
		"pageviews":            "page views",
		"mau":                  "daily, weekly or monthly active users",
		"bounce_rate":          "bounce",
		"top_referrers":        "referrers",
		"by_country":           "geography",
		"conversion_rate":      "conversion",
		"retention_cohorts":    "retention",
		"campaign_results":     "campaign",
		"utm_source":           "referrers",
		"traffic_last_30_days": "website traffic",
	} {
		assert.Equal(t, concept, PrivateAnalyticsTermInKey(key), "key: %q", key)
	}

	for _, key := range []string{
		"rooms_with_conversation", "agent_messages", "anonymous_searches",
		"registered_humans", "count_label", "last_activity_label", "generated_at",
	} {
		assert.Empty(t, PrivateAnalyticsTermInKey(key), "key: %q", key)
	}
}

// The whole serialized payload, walked: no field name and no sentence the API
// wrote carries an audience or growth concept.
func TestPublicOverviewPayload_PublishesNoAudienceOrGrowthVocabulary(t *testing.T) {
	overview := buildPublicOverviewFixture()
	enforcePublicOverviewMetrics(&overview)

	raw, err := json.Marshal(overview)
	require.NoError(t, err)

	var payload any
	require.NoError(t, json.Unmarshal(raw, &payload))

	walkPublicPayload(t, "", payload)
}

// The top-level shape of the public overview is itself an allowlist: a new
// section cannot appear without being named as publishable.
func TestPublicOverviewSections_AreExactlyTheAllowlistedSections(t *testing.T) {
	raw, err := json.Marshal(buildPublicOverviewFixture())
	require.NoError(t, err)

	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &payload))

	for section := range payload {
		purpose, ok := PublicOverviewSections[section]
		assert.True(t, ok, "the overview publishes section %q, which is not allowlisted", section)
		assert.NotEmpty(t, purpose, "section %q must state what it may carry", section)
	}
	for section := range PublicOverviewSections {
		_, ok := payload[section]
		assert.True(t, ok, "the allowlist names section %q, which the overview does not publish", section)
	}
}

// The registration totals are the one figure a reader could mistake for an
// audience. They are labeled as cumulative product accounts and never as
// visitors, active users or traffic.
func TestPublicOverviewCommunityTotals_AreLabeledProductAccountsNotAudience(t *testing.T) {
	overview := buildPublicOverviewFixture()

	byKey := map[string]OverviewMetric{}
	for _, m := range overview.Community.Metrics {
		byKey[m.Key] = m
		assert.Equal(t, allTimeWindowLabel, m.Window, "metric %q", m.Key)
	}

	for _, key := range []string{"registered_agents", "registered_humans"} {
		m := byKey[key]
		require.NotEmpty(t, m.Key, "the all-time section must carry %q", key)
		assert.Contains(t, strings.ToUpper(m.Label), "REGISTERED", "%s label", key)
		assert.NotEmpty(t, m.Qualifier, "%s must state that a registration is not use", key)
		for _, text := range []string{m.Label, m.Definition, m.Qualifier} {
			assert.Empty(t, PrivateAnalyticsTermIn(text), "%s: %q", key, text)
		}
	}
}

// walkPublicPayload checks every field name in the payload, and the text of
// every field the API itself wrote.
func walkPublicPayload(t *testing.T, path string, node any) {
	t.Helper()

	switch v := node.(type) {
	case map[string]any:
		for key, child := range v {
			here := key
			if path != "" {
				here = path + "." + key
			}
			assert.Empty(t, PrivateAnalyticsTermInKey(key), "field %s", here)
			if text, isText := child.(string); isText && PublicOverviewAuthoredText(key) {
				assert.Empty(t, PrivateAnalyticsTermIn(text), "field %s: %q", here, text)
			}
			walkPublicPayload(t, here, child)
		}
	case []any:
		for _, child := range v {
			walkPublicPayload(t, path+"[]", child)
		}
	}
}

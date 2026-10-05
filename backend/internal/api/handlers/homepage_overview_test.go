package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The homepage overview is built entirely by the API: every number carries the
// window it was measured over and the definition of what it counts, every
// excerpt is cut server-side, every relative time is worded server-side, and
// the sparkline arrives pre-normalised so the browser only multiplies by a
// height. These tests pin that contract on the pure builders — no database.

// The search section moved to homepage_search.go when it grew a window
// selector, a chart and its own publishing policy. Its tests live beside it in
// homepage_search_test.go.

// The API-usage section grew a window selector, a series and its own
// measurement at the API boundary. Its tests live beside it in
// homepage_api_usage_test.go; what stays here is its place in the whole
// payload.

// The API-usage section measures CALLS. A registration total sitting among
// them would read as usage, which is the one thing the all-time section exists
// to keep separate.
func TestOverviewAPIUsageSection_CarriesNoRegistrationTotal(t *testing.T) {
	section := buildOverviewAPIUsage(db.APIUsagePulse{Window: db.DefaultRoomStatsWindow()})

	for _, m := range section.Metrics {
		assert.NotContains(t, strings.ToLower(m.Key), "registered",
			"a registration belongs in the all-time section, not among call volumes")
		assert.NotEqual(t, allTimeWindowLabel, m.Window,
			"metric %q: a usage figure is always measured over a window", m.Key)
	}
}

// The all-time section is the scale of Solvr. Its job is to say how big the
// product is WITHOUT letting a registration read as a measure of use.
func TestOverviewAllTimeSection_ShowsScaleWithoutConflatingRegistrationsWithUse(t *testing.T) {
	totals := &db.AllTimeTotals{
		PublicRooms:      214,
		PublishedPosts:   2098,
		RegisteredAgents: 64,
		RegisteredHumans: 1003,
	}
	stats := &db.AllStatsResult{
		TotalContributions: 3327,
		CrystallizedPosts:  12,
	}

	section := buildOverviewCommunity(totals, stats)

	assert.Equal(t, "All time", section.Heading)

	byKey := map[string]OverviewMetric{}
	for _, m := range section.Metrics {
		byKey[m.Key] = m
		assert.Equal(t, allTimeWindowLabel, m.Window, "metric %q", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
		assert.False(t, m.Presence, "metric %q is a total, never a presence reading", m.Key)
		assert.False(t, m.Unavailable, "metric %q was read", m.Key)
	}

	for _, key := range []string{"public_rooms", "published_posts", "registered_agents", "registered_humans"} {
		require.Contains(t, byKey, key, "the all-time section must carry %q", key)
	}
	assert.Equal(t, 214, byKey["public_rooms"].Value)
	assert.Equal(t, 2098, byKey["published_posts"].Value)
	assert.Equal(t, "2,098", byKey["published_posts"].Display)
	assert.Equal(t, 64, byKey["registered_agents"].Value)
	assert.Equal(t, 1003, byKey["registered_humans"].Value)

	// An account total is a REGISTRATION. It may never be worded as "active",
	// which is what would turn it into an audience figure.
	for _, key := range []string{"registered_agents", "registered_humans"} {
		m := byKey[key]
		assert.Contains(t, strings.ToUpper(m.Label), "REGISTERED", "%s label", key)
		assert.NotContains(t, strings.ToLower(m.Label), "active", "%s label", key)
		assert.NotContains(t, strings.ToLower(m.Definition), "active", "%s definition", key)
		assert.NotEmpty(t, m.Qualifier,
			"%s must say out loud that registering is not using", key)
	}

	// Scale that is usage keeps its place beside the registrations.
	assert.Equal(t, 3327, byKey["total_contributions"].Value)
	assert.NotContains(t, byKey, "problems_solved", "the solved metric was retired with the legacy post types (idx 68)")
	// Solvr runs no IPFS node: the page does not count posts as "pinned to IPFS" (the pins of
	// the old node are gone with it).
	assert.NotContains(t, byKey, "crystallized_posts")
	for _, m := range section.Metrics {
		assert.NotContains(t, strings.ToUpper(m.Label+m.Definition), "IPFS", "%s", m.Key)
	}
}

// The two reads behind this section can fail independently, and a failed read
// is not a zero: a zero would claim Solvr is empty.
func TestOverviewAllTimeSection_ReportsEachUnreadTotalSeparately(t *testing.T) {
	registrationKeys := []string{"public_rooms", "published_posts", "registered_agents", "registered_humans"}
	usageKeys := []string{"total_contributions"}

	t.Run("totals unread", func(t *testing.T) {
		section := buildOverviewCommunity(nil, &db.AllStatsResult{TotalContributions: 9})
		byKey := map[string]OverviewMetric{}
		for _, m := range section.Metrics {
			byKey[m.Key] = m
		}
		for _, k := range registrationKeys {
			assert.True(t, byKey[k].Unavailable, "%s was not measured", k)
			assert.Equal(t, "—", byKey[k].Display, "%s must not invent a zero", k)
		}
		for _, k := range usageKeys {
			assert.False(t, byKey[k].Unavailable, "%s was measured", k)
		}
	})

	t.Run("stats unread", func(t *testing.T) {
		section := buildOverviewCommunity(&db.AllTimeTotals{PublicRooms: 7}, nil)
		byKey := map[string]OverviewMetric{}
		for _, m := range section.Metrics {
			byKey[m.Key] = m
		}
		for _, k := range usageKeys {
			assert.True(t, byKey[k].Unavailable, "%s was not measured", k)
			assert.Equal(t, "—", byKey[k].Display, "%s must not invent a zero", k)
		}
		assert.Equal(t, 7, byKey["public_rooms"].Value)
	})

	t.Run("neither read", func(t *testing.T) {
		section := buildOverviewCommunity(nil, nil)
		require.NotEmpty(t, section.Metrics)
		for _, m := range section.Metrics {
			assert.Equal(t, "—", m.Display, "an unread counter says so instead of inventing a zero")
			assert.True(t, m.Unavailable, "metric %q", m.Key)
		}
	})
}

// reusablePosts pads the given posts up to the section's minimum with older ones.
func reusablePosts(lead ...db.ReusablePost) []db.ReusablePost {
	out := append([]db.ReusablePost{}, lead...)
	for i := len(out); i < overviewReusablePostMinimum; i++ {
		out = append(out, db.ReusablePost{ID: fmt.Sprintf("filler-%d", i), Type: "post", Title: "filler",
			ContributionCount: 1, LastActivityAt: time.Now().Add(-72 * time.Hour)})
	}
	return out
}

func TestOverviewPostsSection_LinksReusableKnowledge(t *testing.T) {
	posts := reusablePosts(db.ReusablePost{
		ID:                "11111111-1111-1111-1111-111111111111",
		Type:              "problem",
		Title:             "pgx pool exhausted under load",
		Status:            "solved",
		Tags:              []string{"go", "postgres"},
		ContributionCount: 3,
		LastActivityAt:    time.Now().Add(-48 * time.Hour),
	})

	section := buildOverviewPosts(posts)

	require.NotNil(t, section)
	require.Len(t, section.Items, overviewReusablePostMinimum)
	item := section.Items[0]
	assert.Equal(t, "/posts/11111111-1111-1111-1111-111111111111", item.URL)
	assert.Equal(t, "pgx pool exhausted under load", item.Title)
	assert.NotEmpty(t, item.ContributionLabel)
	assert.Equal(t, "2 days ago", item.LastActivityLabel)
	assert.NotEmpty(t, section.Definition, "the page states what makes a post reusable")
	assert.Equal(t, "/posts", section.BrowseURL)
}

// Below the minimum the section is not published at all: a lone post with one reply is not
// a body of knowledge worth a homepage section (owner decision 2026-10-04, SPEC Part 26).
func TestOverviewPostsSection_IsOmittedBelowTheMinimum(t *testing.T) {
	assert.Equal(t, 3, overviewReusablePostMinimum)
	assert.Nil(t, buildOverviewPosts(nil))
	assert.Nil(t, buildOverviewPosts(reusablePosts()[:overviewReusablePostMinimum-1]))
	assert.NotNil(t, buildOverviewPosts(reusablePosts()))
}

// Every post links to its /posts page, whatever type a row still stores before the legacy
// archive migration relabels it (idx 68).
func TestOverviewPostsSection_LinksEveryPostToItsPostPage(t *testing.T) {
	for _, typ := range []string{"post", "problem", "question", "idea"} {
		section := buildOverviewPosts(reusablePosts(db.ReusablePost{ID: "abc", Type: typ, Title: "t"}))
		require.NotNil(t, section)
		assert.Equal(t, "/posts/abc", section.Items[0].URL, "type %q", typ)
	}
}

func TestOverviewRelativeTime_IsWordedByTheAPI(t *testing.T) {
	now := time.Now()
	cases := []struct {
		at   time.Time
		want string
	}{
		{now.Add(-10 * time.Second), "moments ago"},
		{now.Add(-5 * time.Minute), "5 minutes ago"},
		{now.Add(-1 * time.Minute), "1 minute ago"},
		{now.Add(-1 * time.Hour), "1 hour ago"},
		{now.Add(-5 * time.Hour), "5 hours ago"},
		{now.Add(-26 * time.Hour), "1 day ago"},
		{now.Add(-72 * time.Hour), "3 days ago"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, overviewRelativeTime(c.at), "for %s", c.at)
	}
}

func TestOverviewClosing_EndsOnConnectAgentsNow(t *testing.T) {
	closing := buildOverviewClosing()

	assert.Equal(t, "/connect", closing.ConnectURL)
	assert.Equal(t, "Connect agents now", closing.ConnectLabel)
	assert.NotEmpty(t, closing.Heading)
}

// ---- meta / stale-label coverage (added by task 16) ----

// These tests pin the Task 16 additions to the overview meta and room section:
//   - stale flag + last_updated_label when a partial failure degrades the snapshot
//   - recent completed collaborations returned when no agents are online now
//
// They are pure builder tests — no database needed.

func TestOverviewMeta_StaleLabelPresent(t *testing.T) {
	partialErrors := []string{"search statistics unavailable: simulated failure"}

	meta := buildOverviewMeta(db.DefaultRoomStatsWindow(), partialErrors)

	// A partial error marks the snapshot stale: the data is retained, not fresh.
	assert.True(t, meta.Stale, "partial errors must mark the snapshot stale")
	assert.NotEmpty(t, meta.StaleLabel, "a stale snapshot carries a label the page can show")
}

func TestOverviewMeta_NoStaleWhenNoPartialErrors(t *testing.T) {
	meta := buildOverviewMeta(db.DefaultRoomStatsWindow(), nil)

	assert.False(t, meta.Stale, "no partial errors means a fresh snapshot")
	assert.Empty(t, meta.StaleLabel, "a fresh snapshot has no stale label")
}

func TestBuildStaleLabel_EmptyWhenNoErrors(t *testing.T) {
	assert.Empty(t, buildStaleLabel(nil), "no errors means no stale label")
	assert.Empty(t, buildStaleLabel([]string{}), "empty error slice means no stale label")
}

func TestBuildStaleLabel_SingularAndPlural(t *testing.T) {
	one := buildStaleLabel([]string{"search unavailable"})
	assert.Contains(t, one, "partial refresh")

	many := buildStaleLabel([]string{"search unavailable", "activity unavailable"})
	assert.Contains(t, many, "2 statistics sections")
}

package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The homepage overview is built entirely by the API: every number carries the
// window it was measured over and the definition of what it counts, every
// excerpt is cut server-side, every relative time is worded server-side, and
// the sparkline arrives pre-normalised so the browser only multiplies by a
// height. These tests pin that contract on the pure builders — no database.

func TestOverviewPreviews_OnlyTheAllowListIsShownAndItIsNotRankedByVolume(t *testing.T) {
	// Step 4: prefer the supplied coding example and owner-selected rooms. A
	// loud room that nobody selected must never be promoted by message volume.
	loud := db.PreviewSource{
		Room: models.Room{
			Slug:         "someones-personal-diary",
			DisplayName:  "Personal diary",
			MessageCount: 5000,
			LastActiveAt: time.Now(),
		},
	}
	selected := db.PreviewSource{
		Room: models.Room{
			Slug:         "tictactoe-human-vs-computer-20260920",
			DisplayName:  "Tic Tac Toe",
			Description:  strPtr("Two agents build a game"),
			MessageCount: 9,
			LastActiveAt: time.Now().Add(-25 * time.Hour),
		},
		Participants: []db.RoomParticipant{
			{Name: "planner", AuthorType: "agent", MessageCount: 5},
			{Name: "executor", AuthorType: "agent", MessageCount: 4},
		},
		Exchange: []models.Message{
			{ID: 1, AuthorType: "agent", AgentName: "planner", Content: "do the thing", SequenceNum: intPtr(4)},
			{ID: 2, AuthorType: "agent", AgentName: "executor", Content: "done, here is the evidence", SequenceNum: intPtr(5)},
		},
	}

	section := buildOverviewPreviews([]db.PreviewSource{selected}, []string{selected.Room.Slug})

	require.Len(t, section.Rooms, 1)
	p := section.Rooms[0]
	assert.Equal(t, "tictactoe-human-vs-computer-20260920", p.Slug)
	assert.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920", p.URL)
	assert.Equal(t, "Two agents build a game", p.Purpose)
	assert.Len(t, p.Participants, 2)
	assert.Len(t, p.Exchange, 2)
	assert.NotEmpty(t, p.LastActivityLabel)
	assert.NotEmpty(t, p.SelectedReason, "the page says why this room is here")
	assert.NotEmpty(t, section.Note, "the section states it is editorially selected")

	// The loud room is simply not in the allow-list, so it cannot appear.
	assert.NotContains(t, previewSlugs(buildOverviewPreviews([]db.PreviewSource{selected}, []string{selected.Room.Slug})), loud.Room.Slug)
}

func TestOverviewPreviews_CapsAtThree(t *testing.T) {
	sources := make([]db.PreviewSource, 0, 5)
	slugs := make([]string, 0, 5)
	for _, s := range []string{"room-a", "room-b", "room-c", "room-d", "room-e"} {
		sources = append(sources, db.PreviewSource{Room: models.Room{
			Slug: s, DisplayName: s, LastActiveAt: time.Now(),
		}})
		slugs = append(slugs, s)
	}

	section := buildOverviewPreviews(sources, slugs)

	assert.Len(t, section.Rooms, maxOverviewPreviews)
	assert.Equal(t, 3, maxOverviewPreviews)
}

func TestOverviewPreviews_KeepsTheConfiguredOrderWithTheCodingExampleFirst(t *testing.T) {
	sources := []db.PreviewSource{
		{Room: models.Room{Slug: "room-b", DisplayName: "B", LastActiveAt: time.Now()}},
		{Room: models.Room{Slug: DefaultCollabExampleRoomSlug, DisplayName: "Example", LastActiveAt: time.Now()}},
	}

	section := buildOverviewPreviews(sources, []string{DefaultCollabExampleRoomSlug, "room-b"})

	require.Len(t, section.Rooms, 2)
	assert.Equal(t, DefaultCollabExampleRoomSlug, section.Rooms[0].Slug)
	assert.Equal(t, "room-b", section.Rooms[1].Slug)
}

func TestOverviewPreviewSlugs_DefaultsToTheCodingExample(t *testing.T) {
	assert.Equal(t, []string{DefaultCollabExampleRoomSlug}, parsePreviewSlugs(""))
	assert.Equal(t,
		[]string{"a", "b"},
		parsePreviewSlugs(" a , b ,, "),
		"blank entries are dropped, whitespace trimmed",
	)
	assert.Len(t, parsePreviewSlugs("a,b,c,d,e"), maxOverviewPreviews, "never asks for more than it shows")
}

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
		ProblemsSolved:     41,
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
	assert.Equal(t, 41, byKey["problems_solved"].Value)
	assert.Equal(t, 12, byKey["crystallized_posts"].Value)
}

// The two reads behind this section can fail independently, and a failed read
// is not a zero: a zero would claim Solvr is empty.
func TestOverviewAllTimeSection_ReportsEachUnreadTotalSeparately(t *testing.T) {
	registrationKeys := []string{"public_rooms", "published_posts", "registered_agents", "registered_humans"}
	usageKeys := []string{"total_contributions", "problems_solved", "crystallized_posts"}

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

func TestOverviewPostsSection_LinksReusableKnowledge(t *testing.T) {
	posts := []db.ReusablePost{{
		ID:                "11111111-1111-1111-1111-111111111111",
		Type:              "problem",
		Title:             "pgx pool exhausted under load",
		Status:            "solved",
		Tags:              []string{"go", "postgres"},
		ContributionCount: 3,
		LastActivityAt:    time.Now().Add(-48 * time.Hour),
	}}

	section := buildOverviewPosts(posts)

	require.Len(t, section.Items, 1)
	item := section.Items[0]
	assert.Equal(t, "/problems/11111111-1111-1111-1111-111111111111", item.URL)
	assert.Equal(t, "pgx pool exhausted under load", item.Title)
	assert.NotEmpty(t, item.ContributionLabel)
	assert.Equal(t, "2 days ago", item.LastActivityLabel)
	assert.NotEmpty(t, section.Definition, "the page states what makes a post reusable")
	assert.Equal(t, "/posts", section.BrowseURL)
}

func TestOverviewPostsSection_RoutesEachTypeToItsOwnPage(t *testing.T) {
	for typ, want := range map[string]string{
		"problem":  "/problems/abc",
		"question": "/questions/abc",
		"idea":     "/ideas/abc",
		"other":    "/posts/abc",
	} {
		section := buildOverviewPosts([]db.ReusablePost{{ID: "abc", Type: typ, Title: "t"}})
		require.Len(t, section.Items, 1)
		assert.Equal(t, want, section.Items[0].URL, "type %q", typ)
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

func previewSlugs(section OverviewPreviews) []string {
	out := make([]string, 0, len(section.Rooms))
	for _, r := range section.Rooms {
		out = append(out, r.Slug)
	}
	return out
}

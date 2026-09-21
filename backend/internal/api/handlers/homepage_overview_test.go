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

func overviewActivityRows(n int) []db.PublicRoomActivity {
	rows := make([]db.PublicRoomActivity, 0, n)
	base := time.Now().Add(-2 * time.Hour)
	for i := 0; i < n; i++ {
		seq := i + 1
		rows = append(rows, db.PublicRoomActivity{
			MessageID:   int64(i + 1),
			SequenceNum: &seq,
			RoomSlug:    "room-alpha",
			RoomName:    "Room Alpha",
			AuthorType:  "agent",
			AgentName:   "planner_one",
			Content:     "a short line",
			CreatedAt:   base.Add(time.Duration(i) * time.Minute),
		})
	}
	return rows
}

func TestOverviewActivity_SixItemsAndLoadMore(t *testing.T) {
	// The repo is asked for limit+1 rows so the API — not the browser — decides
	// whether there is more to load.
	activity := buildOverviewActivity(overviewActivityRows(7), 6, 0)

	require.Len(t, activity.Items, 6, "the homepage shows six recent activities")
	assert.Equal(t, 6, activity.Limit)
	assert.Equal(t, 0, activity.Offset)
	assert.True(t, activity.HasMore)
	assert.Equal(t, 6, activity.NextOffset)
	assert.NotEmpty(t, activity.LoadMoreLabel)
	assert.Equal(t, "/v1/homepage/activity?offset=6&limit=6", activity.LoadMoreURL)
	assert.NotEmpty(t, activity.Definition)
}

func TestOverviewActivity_NoMoreWhenRepoReturnsExactlyTheLimit(t *testing.T) {
	activity := buildOverviewActivity(overviewActivityRows(6), 6, 12)

	require.Len(t, activity.Items, 6)
	assert.False(t, activity.HasMore)
	assert.Equal(t, 0, activity.NextOffset)
	assert.Empty(t, activity.LoadMoreURL)
	assert.Equal(t, 12, activity.Offset)
}

func TestOverviewActivity_ItemsCarryLinksRolesAndAPIWordedTime(t *testing.T) {
	seq := 6
	rows := []db.PublicRoomActivity{{
		MessageID:   99,
		SequenceNum: &seq,
		RoomSlug:    "tictactoe-human-vs-computer-20260920",
		RoomName:    "Tic Tac Toe",
		AuthorType:  "human",
		AgentName:   "felipe",
		Content:     "short",
		CreatedAt:   time.Now().Add(-3 * time.Hour),
	}}

	activity := buildOverviewActivity(rows, 6, 0)

	require.Len(t, activity.Items, 1)
	item := activity.Items[0]
	assert.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920", item.RoomURL)
	assert.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920#message-6", item.MessageURL)
	assert.Equal(t, "human", item.AuthorRole)
	assert.Equal(t, "felipe", item.Author)
	assert.Equal(t, "3 hours ago", item.TimeLabel, "the API words the relative time")
	assert.False(t, item.IsExcerpt)
	assert.Empty(t, item.ExcerptNote)
}

func TestOverviewActivity_LongMessageIsExcerptedAndSaysSo(t *testing.T) {
	long := strings.Repeat("alpha beta ", 60) // ~660 chars
	rows := []db.PublicRoomActivity{{
		MessageID:  1,
		RoomSlug:   "room-alpha",
		RoomName:   "Room Alpha",
		AuthorType: "agent",
		AgentName:  "executor_one",
		Content:    long,
		CreatedAt:  time.Now(),
	}}

	activity := buildOverviewActivity(rows, 6, 0)

	item := activity.Items[0]
	assert.True(t, item.IsExcerpt)
	assert.NotEmpty(t, item.ExcerptNote)
	assert.Less(t, len([]rune(item.Excerpt)), len([]rune(long)))
	assert.True(t, strings.HasSuffix(item.Excerpt, "…"), "excerpt: %q", item.Excerpt)
	assert.Empty(t, item.MessageURL, "no sequence_num means no anchor to link to")
}

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

func TestOverviewSearchSection_StatesItsWindowsAndFormatsTheRate(t *testing.T) {
	summary := models.SearchAnalytics{
		TotalSearches:  200,
		UniqueQueries:  75,
		AvgDurationMs:  42.6,
		ZeroResultRate: 0.4925,
		BySearcherType: map[string]int{"agent": 150, "human": 50},
	}
	trending := []models.TrendingSearch{{Query: "postgres race condition", Count: 12}}
	recent := []db.RecentQuery{{
		Query:        "vitest mock hoisting",
		Occurrences:  3,
		ResultsCount: 2,
		LastSearched: time.Now().Add(-90 * time.Second),
	}}
	categories := []models.DataCategory{{Category: "problem", SearchCount: 30}}

	section := buildOverviewSearch(summary, trending, recent, categories)

	require.NotEmpty(t, section.Metrics)
	for _, m := range section.Metrics {
		assert.NotEmpty(t, m.Window, "metric %q window", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
		assert.NotEmpty(t, m.Display, "metric %q display", m.Key)
	}
	byKey := map[string]OverviewMetric{}
	for _, m := range section.Metrics {
		byKey[m.Key] = m
	}
	assert.Equal(t, "49%", byKey["zero_result_rate_7d"].Display,
		"the API formats the percentage; the browser does no arithmetic")
	assert.Equal(t, "43 ms", byKey["avg_search_duration_7d"].Display)
	assert.Equal(t, 150, byKey["agent_searches_7d"].Value)

	require.Len(t, section.Trending.Rows, 1)
	assert.Equal(t, "postgres race condition", section.Trending.Rows[0].Label)
	assert.NotEmpty(t, section.Trending.Rows[0].CountLabel)
	assert.NotEmpty(t, section.Trending.Definition)

	require.Len(t, section.Recent.Rows, 1)
	assert.Equal(t, "vitest mock hoisting", section.Recent.Rows[0].Label)
	assert.NotEmpty(t, section.Recent.Rows[0].TimeLabel)
	assert.Contains(t, section.Recent.Definition, "more than once",
		"the page states the privacy rule it applies to recent queries")

	require.Len(t, section.Categories.Rows, 1)
	assert.Equal(t, "problem", section.Categories.Rows[0].Label)
}

func TestOverviewAPIUsageSection_OnlyServesMeasuredNumbers(t *testing.T) {
	summary := models.SearchAnalytics{
		TotalSearches:  200,
		BySearcherType: map[string]int{"agent": 150, "human": 50},
	}
	pulse := db.RoomPulse{Messages24h: 321}

	section := buildOverviewAPIUsage(summary, pulse, 64)

	require.NotEmpty(t, section.Metrics)
	for _, m := range section.Metrics {
		assert.NotEmpty(t, m.Window, "metric %q window", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
	}
	byKey := map[string]OverviewMetric{}
	for _, m := range section.Metrics {
		byKey[m.Key] = m
	}
	assert.Equal(t, 150, byKey["agent_searches_7d"].Value)
	assert.Equal(t, 321, byKey["room_messages_24h"].Value)
	assert.Equal(t, 64, byKey["registered_agents"].Value)
	assert.NotEmpty(t, section.Endpoints)
	for _, e := range section.Endpoints {
		assert.NotEmpty(t, e.Method)
		assert.True(t, strings.HasPrefix(e.Path, "/v1/"), "endpoint path %q", e.Path)
		assert.NotEmpty(t, e.Summary)
	}
	assert.Equal(t, "/api-docs", section.DocsURL)
}

func TestOverviewCommunitySection_IsAllTimeAndDefined(t *testing.T) {
	stats := &db.AllStatsResult{
		TotalPosts:         2098,
		TotalContributions: 3327,
		TotalAgents:        64,
		HumansCount:        1003,
		ProblemsSolved:     41,
		CrystallizedPosts:  12,
	}

	section := buildOverviewCommunity(stats)

	require.Len(t, section.Metrics, 6)
	for _, m := range section.Metrics {
		assert.Equal(t, allTimeWindowLabel, m.Window, "metric %q", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
	}
}

func TestOverviewCommunitySection_SurvivesAMissingStatsRead(t *testing.T) {
	section := buildOverviewCommunity(nil)

	require.NotEmpty(t, section.Metrics)
	for _, m := range section.Metrics {
		assert.Equal(t, "—", m.Display, "an unread counter says so instead of inventing a zero")
	}
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

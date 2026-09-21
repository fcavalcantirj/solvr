package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /v1/homepage/overview and GET /v1/homepage/activity end to end, against a
// real database, called the way a logged-out browser calls them.
//
// What these tests are really guarding is the privacy boundary of a page that
// anyone can read: a private room, a family-private post and a one-off search
// term must not appear ANYWHERE in the raw response body, and a room may only
// be previewed because someone chose it — never because it is busy.

type hpoMetric struct {
	Key        string `json:"key"`
	Label      string `json:"label"`
	Value      int    `json:"value"`
	Display    string `json:"display"`
	Window     string `json:"window"`
	Definition string `json:"definition"`
}

type hpoTableRow struct {
	Label      string `json:"label"`
	Value      int    `json:"value"`
	CountLabel string `json:"count_label"`
	TimeLabel  string `json:"time_label"`
}

type hpoTable struct {
	Heading    string        `json:"heading"`
	Window     string        `json:"window"`
	Definition string        `json:"definition"`
	Rows       []hpoTableRow `json:"rows"`
}

type hpoActivityItem struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	RoomSlug     string `json:"room_slug"`
	RoomName     string `json:"room_name"`
	RoomURL      string `json:"room_url"`
	Author       string `json:"author"`
	AuthorRole   string `json:"author_role"`
	AuthorLabel  string `json:"author_label"`
	AuthorNote   string `json:"author_note"`
	Action       string `json:"action"`
	ActionStated bool   `json:"action_stated"`
	Excerpt      string `json:"excerpt"`
	LinkURL      string `json:"link_url"`
	LinkLabel    string `json:"link_label"`
	TimeLabel    string `json:"time_label"`
}

type hpoActivityGroup struct {
	RoomSlug   string            `json:"room_slug"`
	RoomName   string            `json:"room_name"`
	RoomURL    string            `json:"room_url"`
	TimeLabel  string            `json:"time_label"`
	EntryCount int               `json:"entry_count"`
	CountLabel string            `json:"count_label"`
	BurstNote  string            `json:"burst_note"`
	Items      []hpoActivityItem `json:"items"`
}

type hpoActivity struct {
	Heading       string             `json:"heading"`
	Definition    string             `json:"definition"`
	OutcomeNote   string             `json:"outcome_note"`
	Groups        []hpoActivityGroup `json:"groups"`
	EntryCount    int                `json:"entry_count"`
	Limit         int                `json:"limit"`
	Offset        int                `json:"offset"`
	NextOffset    int                `json:"next_offset"`
	HasMore       bool               `json:"has_more"`
	LoadMoreLabel string             `json:"load_more_label"`
	LoadMoreURL   string             `json:"load_more_url"`
	Cursor        string             `json:"cursor"`
	RefreshURL    string             `json:"refresh_url"`
	HasNew        bool               `json:"has_new"`
	NewCount      int                `json:"new_count"`
	NewLabel      string             `json:"new_label"`
}

// entries flattens the grouped stream back into one list, for the assertions
// that are about the entries themselves rather than about how they are grouped.
func (a hpoActivity) entries() []hpoActivityItem {
	out := make([]hpoActivityItem, 0, a.EntryCount)
	for _, g := range a.Groups {
		out = append(out, g.Items...)
	}
	return out
}

type hpoPreview struct {
	Slug         string `json:"slug"`
	DisplayName  string `json:"display_name"`
	URL          string `json:"url"`
	Purpose      string `json:"purpose"`
	Participants []struct {
		Name         string `json:"name"`
		Role         string `json:"role"`
		MessageLabel string `json:"message_label"`
	} `json:"participants"`
	Exchange []struct {
		Author     string `json:"author"`
		Excerpt    string `json:"excerpt"`
		MessageURL string `json:"message_url"`
	} `json:"exchange"`
	MessageCount      int    `json:"message_count"`
	LastActivityLabel string `json:"last_activity_label"`
	SelectedReason    string `json:"selected_reason"`
}

type hpoOverview struct {
	Rooms struct {
		Heading         string      `json:"heading"`
		ScopeLabel      string      `json:"scope_label"`
		PresenceHeading string      `json:"presence_heading"`
		PresenceMetrics []hpoMetric `json:"presence_metrics"`
		SelectedWindow  string      `json:"selected_window"`
		WindowOptions   []struct {
			Value    string `json:"value"`
			Selected bool   `json:"selected"`
		} `json:"window_options"`
		Metrics   []hpoMetric `json:"metrics"`
		Sparkline *struct {
			Label      string `json:"label"`
			Window     string `json:"window"`
			Definition string `json:"definition"`
			MaxValue   int    `json:"max_value"`
			Points     []struct {
				Label      string  `json:"label"`
				Value      int     `json:"value"`
				Normalized float64 `json:"normalized"`
			} `json:"points"`
		} `json:"sparkline"`
		RoomsURL string `json:"rooms_url"`
	} `json:"rooms"`
	Previews struct {
		Heading string       `json:"heading"`
		Note    string       `json:"note"`
		Rooms   []hpoPreview `json:"rooms"`
	} `json:"previews"`
	APIUsage struct {
		Metrics   []hpoMetric `json:"metrics"`
		Endpoints []struct {
			Method  string `json:"method"`
			Path    string `json:"path"`
			Summary string `json:"summary"`
		} `json:"endpoints"`
		DocsURL string `json:"docs_url"`
	} `json:"api_usage"`
	Search struct {
		Metrics    []hpoMetric `json:"metrics"`
		Trending   hpoTable    `json:"trending"`
		Recent     hpoTable    `json:"recent"`
		Categories hpoTable    `json:"categories"`
	} `json:"search"`
	Community struct {
		Metrics []hpoMetric `json:"metrics"`
	} `json:"community"`
	Posts struct {
		Heading    string `json:"heading"`
		Definition string `json:"definition"`
		Items      []struct {
			ID                string `json:"id"`
			Type              string `json:"type"`
			Title             string `json:"title"`
			URL               string `json:"url"`
			ContributionCount int    `json:"contribution_count"`
			ContributionLabel string `json:"contribution_label"`
			LastActivityLabel string `json:"last_activity_label"`
		} `json:"items"`
		BrowseURL string `json:"browse_url"`
	} `json:"posts"`
	Closing struct {
		Heading      string `json:"heading"`
		ConnectURL   string `json:"connect_url"`
		ConnectLabel string `json:"connect_label"`
	} `json:"closing"`
	ActivityRaw hpoActivity `json:"activity"`
}

// getHomepageOverview calls the overview with no credentials at all.
func getHomepageOverview(t *testing.T, baseURL string) (hpoOverview, string) {
	t.Helper()
	resp, err := http.Get(baseURL + "/v1/homepage/overview")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data hpoOverview `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

// getHomepageActivity calls the Load more endpoint with no credentials.
func getHomepageActivity(t *testing.T, baseURL string, offset, limit int) (hpoActivity, string) {
	t.Helper()
	url := fmt.Sprintf("%s/v1/homepage/activity?offset=%d&limit=%d", baseURL, offset, limit)
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data hpoActivity `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

// hpoSeedRoom creates a room and fills it with an alternating two-agent
// transcript, newest message last.
func hpoSeedRoom(t *testing.T, pool *db.Pool, slug, name, purpose string, private bool, messages []string) *models.Room {
	t.Helper()
	ctx := context.Background()
	roomRepo := db.NewRoomRepository(pool)
	msgRepo := db.NewMessageRepository(pool)

	desc := purpose
	room, _, err := roomRepo.Create(ctx, models.CreateRoomParams{
		Slug:        slug,
		DisplayName: name,
		Description: &desc,
		IsPrivate:   private,
		OwnerID:     uuid.Nil,
	})
	require.NoError(t, err)

	authors := []string{"hpo_planner", "hpo_executor"}
	for i, content := range messages {
		_, err := msgRepo.Create(ctx, models.CreateMessageParams{
			RoomID:      room.ID,
			AuthorType:  "agent",
			AgentName:   authors[i%2],
			Content:     content,
			ContentType: "text",
		})
		require.NoError(t, err)
		require.NoError(t, roomRepo.IncrementMessageCount(ctx, room.ID))
	}

	refreshed, err := roomRepo.GetBySlug(ctx, slug)
	require.NoError(t, err)
	return refreshed
}

// hpoInsertSearches records the same normalised term n times.
func hpoInsertSearches(t *testing.T, pool *db.Pool, query string, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := pool.Exec(context.Background(),
			`INSERT INTO search_queries
			   (query, query_normalized, results_count, search_method, duration_ms, searcher_type, searched_at)
			 VALUES ($1, $1, 2, 'hybrid', 12, 'agent', NOW())`, query)
		require.NoError(t, err)
	}
}

// hpoInsertPostWithApproach creates a post at the given visibility plus one
// approach on it, so the post qualifies as reusable knowledge.
func hpoInsertPostWithApproach(t *testing.T, pool *db.Pool, title, visibility string) string {
	t.Helper()
	ctx := context.Background()
	postID := uuid.New().String()

	_, err := pool.Exec(ctx,
		`INSERT INTO posts (id, type, title, description, tags, posted_by_type, posted_by_id, status, visibility)
		 VALUES ($1, 'problem', $2, 'seeded by the homepage overview test', ARRAY['hpo'], 'agent', 'agent_hpotest', 'open', $3)`,
		postID, title, visibility,
	)
	require.NoError(t, err)

	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', 'agent_hpotest', 'seeded approach', 'working')`,
		postID,
	)
	require.NoError(t, err)

	return postID
}

// hpoCleanup removes this file's fixtures. Registered AFTER the shared cleanup
// so LIFO runs it BEFORE pool.Close() — deferred funcs run before t.Cleanup
// funcs, and a closed pool swallows every delete.
func hpoCleanup(t *testing.T, pool *db.Pool) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		"DELETE FROM approaches WHERE author_id = 'agent_hpotest'",
		"DELETE FROM posts WHERE posted_by_id = 'agent_hpotest'",
		"DELETE FROM search_queries WHERE query_normalized LIKE 'hpo %'",
		"DELETE FROM messages WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-hpo-%')",
		"DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE 'test-hpo-%')",
		"DELETE FROM rooms WHERE slug LIKE 'test-hpo-%'",
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Logf("hpoCleanup: %q failed: %v", stmt, err)
		}
	}
}

func hpoSlug(suffix string) string {
	return fmt.Sprintf("test-hpo-%s-%d", suffix, time.Now().UnixNano()%1000000)
}

func TestHomepageOverview_ServesEverySectionToALoggedOutVisitor(t *testing.T) {
	previewSlug := hpoSlug("preview")
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", previewSlug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, previewSlug, "Overview Preview Room", "Two agents build a game", false, []string{
		"PLANNER ONLINE. Correct the demo into one human versus a deterministic computer.",
		"EXECUTOR ONLINE. Plan posted before editing anything.",
		"PLAN APPROVED. Keep the fixed minimax tie-break.",
		"IMPLEMENTATION COMPLETE. 8 tests pass, 0 fail.",
	})
	hpoInsertSearches(t, pool, "hpo postgres race condition", 3)
	publicTitle := "hpo public reusable problem"
	hpoInsertPostWithApproach(t, pool, publicTitle, "public")

	ov, raw := getHomepageOverview(t, ts.URL)

	// --- live room statistics -------------------------------------------
	require.NotEmpty(t, ov.Rooms.Heading, "raw: %s", raw)
	assert.Equal(t, "Public room activity", ov.Rooms.ScopeLabel)
	require.Len(t, ov.Rooms.PresenceMetrics, 2, "the two Now figures")
	require.Len(t, ov.Rooms.Metrics, 4, "the four windowed figures")
	for _, m := range append(append([]hpoMetric{}, ov.Rooms.PresenceMetrics...), ov.Rooms.Metrics...) {
		assert.NotEmpty(t, m.Window, "room metric %q must state its window", m.Key)
		assert.NotEmpty(t, m.Definition, "room metric %q must state its definition", m.Key)
		assert.NotEmpty(t, m.Display, "room metric %q display", m.Key)
	}
	assert.Equal(t, "Now", ov.Rooms.PresenceHeading)
	for _, m := range ov.Rooms.PresenceMetrics {
		assert.Equal(t, "now", m.Window, "presence metric %q is never windowed", m.Key)
	}
	assert.Equal(t, "24h", ov.Rooms.SelectedWindow, "the index opens on 24 hours")
	require.Len(t, ov.Rooms.WindowOptions, 3)
	assert.Equal(t, "/rooms", ov.Rooms.RoomsURL)

	require.NotNil(t, ov.Rooms.Sparkline, "the room section carries its series")
	require.Len(t, ov.Rooms.Sparkline.Points, 24, "one bucket per hour")
	assert.NotEmpty(t, ov.Rooms.Sparkline.Definition)
	for _, p := range ov.Rooms.Sparkline.Points {
		assert.GreaterOrEqual(t, p.Normalized, 0.0)
		assert.LessOrEqual(t, p.Normalized, 1.0, "the API normalises; the client only scales")
	}

	// --- activity stream ------------------------------------------------
	require.NotEmpty(t, ov.ActivityRaw.entries(), "the seeded room's messages are activity")
	assert.Equal(t, 6, ov.ActivityRaw.Limit)
	assert.Equal(t, "Load more", ov.ActivityRaw.LoadMoreLabel)
	assert.NotEmpty(t, ov.ActivityRaw.Definition)
	found := false
	for _, item := range ov.ActivityRaw.entries() {
		if item.RoomSlug != previewSlug {
			continue
		}
		found = true
		assert.Equal(t, "/rooms/"+previewSlug, item.RoomURL)
		assert.Contains(t, item.LinkURL, "#message-")
		assert.NotEmpty(t, item.TimeLabel, "the API words the relative time")
		assert.Equal(t, "agent", item.AuthorRole)
		assert.Equal(t, "Agent", item.AuthorLabel)
		assert.NotEmpty(t, item.Action, "every entry says what it was")
	}
	assert.True(t, found, "the seeded public room appears in the stream")

	// --- editorial previews ---------------------------------------------
	require.Len(t, ov.Previews.Rooms, 1)
	preview := ov.Previews.Rooms[0]
	assert.Equal(t, previewSlug, preview.Slug)
	assert.Equal(t, "/rooms/"+previewSlug, preview.URL)
	assert.Equal(t, "Two agents build a game", preview.Purpose, "purpose comes from the room")
	assert.Len(t, preview.Participants, 2, "both agents are named")
	require.Len(t, preview.Exchange, 2, "a preview shows a real back-and-forth")
	assert.NotEqual(t, preview.Exchange[0].Author, preview.Exchange[1].Author)
	assert.NotEmpty(t, preview.LastActivityLabel)
	assert.NotEmpty(t, preview.SelectedReason)
	assert.NotEmpty(t, ov.Previews.Note)

	// --- API usage ------------------------------------------------------
	require.NotEmpty(t, ov.APIUsage.Metrics)
	for _, m := range ov.APIUsage.Metrics {
		assert.NotEmpty(t, m.Window, "api metric %q window", m.Key)
		assert.NotEmpty(t, m.Definition, "api metric %q definition", m.Key)
	}
	require.NotEmpty(t, ov.APIUsage.Endpoints)
	assert.Equal(t, "/api-docs", ov.APIUsage.DocsURL)

	// --- search statistics ----------------------------------------------
	require.NotEmpty(t, ov.Search.Metrics)
	for _, m := range ov.Search.Metrics {
		assert.NotEmpty(t, m.Window, "search metric %q window", m.Key)
		assert.NotEmpty(t, m.Definition, "search metric %q definition", m.Key)
	}
	assert.NotEmpty(t, ov.Search.Trending.Definition)
	assert.NotEmpty(t, ov.Search.Recent.Definition)
	assert.NotEmpty(t, ov.Search.Categories.Definition)
	trendingTerms := make([]string, 0, len(ov.Search.Trending.Rows))
	for _, row := range ov.Search.Trending.Rows {
		trendingTerms = append(trendingTerms, row.Label)
	}
	assert.Contains(t, trendingTerms, "hpo postgres race condition")

	// --- all-time community totals --------------------------------------
	require.Len(t, ov.Community.Metrics, 6)
	for _, m := range ov.Community.Metrics {
		assert.Equal(t, "all time", m.Window, "community metric %q", m.Key)
		assert.NotEmpty(t, m.Definition, "community metric %q definition", m.Key)
	}

	// --- reusable posts --------------------------------------------------
	assert.NotEmpty(t, ov.Posts.Definition, "the page states what makes a post reusable")
	assert.Equal(t, "/posts", ov.Posts.BrowseURL)
	postTitles := make([]string, 0, len(ov.Posts.Items))
	for _, item := range ov.Posts.Items {
		postTitles = append(postTitles, item.Title)
		assert.Positive(t, item.ContributionCount, "a reusable post carries contributions")
		assert.NotEmpty(t, item.URL)
	}
	assert.Contains(t, postTitles, publicTitle)

	// --- the page closes on Connect agents now ---------------------------
	assert.Equal(t, "/connect", ov.Closing.ConnectURL)
	assert.Equal(t, "Connect agents now", ov.Closing.ConnectLabel)
}

func TestHomepageOverview_NeverLeaksAPrivateRoom(t *testing.T) {
	publicSlug := hpoSlug("pub")
	privateSlug := hpoSlug("priv")
	secret := "CONFIDENTIAL PRIVATE ROOM CONTENT that must never reach the homepage"
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", privateSlug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, publicSlug, "Public Room", "readable by anyone", false, []string{
		"a public message", "another public message",
	})
	hpoSeedRoom(t, pool, privateSlug, "Private Room", "closed", true, []string{
		secret, secret + " again",
	})

	_, raw := getHomepageOverview(t, ts.URL)

	assert.NotContains(t, raw, privateSlug, "a private room's slug must not appear")
	assert.NotContains(t, raw, secret, "a private room's content must not appear")
	assert.NotContains(t, raw, "Private Room", "a private room's name must not appear")
	assert.Contains(t, raw, publicSlug, "the public room still appears")

	// Even when it is the ONLY configured preview, a private room yields no preview.
	ov, _ := getHomepageOverview(t, ts.URL)
	assert.Empty(t, ov.Previews.Rooms, "a private room is never previewed")
}

func TestHomepageOverview_PreviewsHonourTheAllowListAndIgnoreVolume(t *testing.T) {
	selectedSlug := hpoSlug("chosen")
	loudSlug := hpoSlug("loud")
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", selectedSlug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	hpoSeedRoom(t, pool, selectedSlug, "The Chosen Room", "an editorial pick", false, []string{
		"planner speaks", "executor answers",
	})

	// A public room with far more traffic that nobody selected. Step 4: volume
	// is not a reason to promote a room — the loudest rooms are the most likely
	// to be personal.
	loudMessages := make([]string, 0, 40)
	for i := 0; i < 40; i++ {
		loudMessages = append(loudMessages, fmt.Sprintf("loud chatter number %d", i))
	}
	loud := hpoSeedRoom(t, pool, loudSlug, "Very Busy Room", "lots of noise", false, loudMessages)
	require.Equal(t, 40, loud.MessageCount)

	ov, _ := getHomepageOverview(t, ts.URL)

	require.Len(t, ov.Previews.Rooms, 1, "only the allow-listed room is previewed")
	assert.Equal(t, selectedSlug, ov.Previews.Rooms[0].Slug)
	previewSlugs := []string{ov.Previews.Rooms[0].Slug}
	assert.NotContains(t, previewSlugs, loudSlug,
		"the busiest room must not be promoted into a preview")
}

func TestHomepageOverview_PreviewDisappearsWhenTheRoomGoesPrivate(t *testing.T) {
	slug := hpoSlug("flip")
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	room := hpoSeedRoom(t, pool, slug, "Flipping Room", "public for now", false, []string{
		"planner speaks here", "executor answers here",
	})

	before, _ := getHomepageOverview(t, ts.URL)
	require.Len(t, before.Previews.Rooms, 1)

	private := true
	_, err := db.NewRoomRepository(pool).Update(context.Background(), room.ID, models.UpdateRoomParams{
		IsPrivate: &private,
	})
	require.NoError(t, err)

	after, raw := getHomepageOverview(t, ts.URL)
	assert.Empty(t, after.Previews.Rooms, "the preview is gone the moment the room closes")
	assert.NotContains(t, raw, slug, "no trace of the room survives anywhere on the page")
	assert.NotContains(t, raw, "planner speaks here")
}

func TestHomepageActivity_PaginatesBehindLoadMore(t *testing.T) {
	slug := hpoSlug("page")
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", slug)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	messages := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		messages = append(messages, fmt.Sprintf("paged message %02d", i))
	}
	hpoSeedRoom(t, pool, slug, "Paged Room", "pagination fixture", false, messages)

	first, _ := getHomepageActivity(t, ts.URL, 0, 6)
	require.Len(t, first.entries(), 6)
	assert.True(t, first.HasMore)
	assert.Equal(t, 6, first.NextOffset)
	assert.Equal(t, "/v1/homepage/activity?offset=6&limit=6", first.LoadMoreURL)

	second, _ := getHomepageActivity(t, ts.URL, first.NextOffset, 6)
	assert.Equal(t, 6, second.Offset)
	require.NotEmpty(t, second.entries())

	// Load more shows different messages, not the same page again.
	firstExcerpts := map[string]bool{}
	for _, item := range first.entries() {
		firstExcerpts[item.Excerpt] = true
	}
	overlap := 0
	for _, item := range second.entries() {
		if firstExcerpts[item.Excerpt] {
			overlap++
		}
	}
	assert.Zero(t, overlap, "the second page repeats nothing from the first")
}

func TestHomepageOverview_RecentQueriesOnlyShowRepeatedTerms(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("none"))

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	oneOff := "hpo my private medical question"
	repeated := "hpo vitest mock hoisting"
	hpoInsertSearches(t, pool, oneOff, 1)
	hpoInsertSearches(t, pool, repeated, 3)

	ov, raw := getHomepageOverview(t, ts.URL)

	assert.NotContains(t, raw, oneOff,
		"a term searched once belongs to one visitor and is never published")

	recentTerms := make([]string, 0, len(ov.Search.Recent.Rows))
	for _, row := range ov.Search.Recent.Rows {
		recentTerms = append(recentTerms, row.Label)
		assert.NotEmpty(t, row.TimeLabel)
		assert.GreaterOrEqual(t, row.Value, 2, "every published term was searched more than once")
	}
	assert.Contains(t, recentTerms, repeated)
	assert.Contains(t, strings.ToLower(ov.Search.Recent.Definition), "more than once",
		"the page states the rule it applies")
}

func TestHomepageOverview_ReusablePostsExcludeFamilyPrivatePosts(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("none"))

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	publicTitle := "hpo public problem with an approach"
	familyTitle := "hpo family private problem nobody may see"
	hpoInsertPostWithApproach(t, pool, publicTitle, "public")
	hpoInsertPostWithApproach(t, pool, familyTitle, "family")

	ov, raw := getHomepageOverview(t, ts.URL)

	assert.NotContains(t, raw, familyTitle, "a family-private post must not reach the homepage")

	titles := make([]string, 0, len(ov.Posts.Items))
	for _, item := range ov.Posts.Items {
		titles = append(titles, item.Title)
	}
	assert.Contains(t, titles, publicTitle)
}

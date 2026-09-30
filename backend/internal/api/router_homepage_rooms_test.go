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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// GET /v1/homepage/rooms end to end, against a real database, called the way a
// logged-out browser calls it when someone moves the time-window selector.
//
// The point of these tests is that the page cannot lie: a presence number is
// stated as "now" and does not move when the window does, a historical number
// states the window it was measured over, and a milestone nobody has recorded
// reads as unavailable rather than as a confident zero.

type hprMetric struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Value       int    `json:"value"`
	Display     string `json:"display"`
	Window      string `json:"window"`
	Definition  string `json:"definition"`
	Presence    bool   `json:"presence"`
	Unavailable bool   `json:"unavailable"`
	Qualifier   string `json:"qualifier"`
}

type hprWindowOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
}

type hprRooms struct {
	Heading         string            `json:"heading"`
	ScopeLabel      string            `json:"scope_label"`
	ScopeNote       string            `json:"scope_note"`
	PresenceHeading string            `json:"presence_heading"`
	PresenceNote    string            `json:"presence_note"`
	PresenceMetrics []hprMetric       `json:"presence_metrics"`
	WindowHeading   string            `json:"window_heading"`
	WindowLabel     string            `json:"window_label"`
	WindowOptions   []hprWindowOption `json:"window_options"`
	SelectedWindow  string            `json:"selected_window"`
	Metrics         []hprMetric       `json:"metrics"`
	Sparkline       *struct {
		Label  string `json:"label"`
		Window string `json:"window"`
		Points []struct {
			Label  string `json:"label"`
			Value  int    `json:"value"`
			Height string `json:"height"`
		} `json:"points"`
	} `json:"sparkline"`
	RoomsURL string `json:"rooms_url"`
}

// getHomepageRooms calls the room statistics with no credentials at all.
func getHomepageRooms(t *testing.T, baseURL, window string) (hprRooms, string) {
	t.Helper()
	url := baseURL + "/v1/homepage/rooms"
	if window != "" {
		url += "?window=" + window
	}
	resp, err := http.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body: %s", string(body))

	var wrapper struct {
		Data hprRooms `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &wrapper), "body: %s", string(body))
	return wrapper.Data, string(body)
}

func hprMetricByKey(t *testing.T, metrics []hprMetric, key string) hprMetric {
	t.Helper()
	for _, m := range metrics {
		if m.Key == key {
			return m
		}
	}
	t.Fatalf("metric %q not found", key)
	return hprMetric{}
}

func TestHomepageRooms_ServesTheSixStatisticsToALoggedOutVisitor(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	slug := hpoSlug("stats")
	hpoSeedRoom(t, pool, slug, "Room Stats", "Two agents work", false, []string{
		"PLANNER ONLINE.",
		"EXECUTOR ONLINE.",
	})

	rooms, raw := getHomepageRooms(t, ts.URL, "")

	// The counts cover every room, private ones included (spec.json idx 96).
	assert.Equal(t, "All rooms, private ones included", rooms.ScopeLabel, "raw: %s", raw)
	assert.NotEmpty(t, rooms.ScopeNote)
	assert.Equal(t, "/rooms", rooms.RoomsURL)

	presenceKeys := make([]string, 0, len(rooms.PresenceMetrics))
	for _, m := range rooms.PresenceMetrics {
		presenceKeys = append(presenceKeys, m.Key)
	}
	assert.Equal(t, []string{"agents_online_now", "rooms_with_agents_online_now"}, presenceKeys)

	windowedKeys := make([]string, 0, len(rooms.Metrics))
	for _, m := range rooms.Metrics {
		windowedKeys = append(windowedKeys, m.Key)
	}
	assert.Equal(t, []string{
		"rooms_with_conversation", "agent_messages", "human_messages", "rooms_with_two_way_exchanges",
	}, windowedKeys)

	for _, m := range append(append([]hprMetric{}, rooms.PresenceMetrics...), rooms.Metrics...) {
		assert.NotEmpty(t, m.Label, "metric %q label", m.Key)
		assert.NotEmpty(t, m.Display, "metric %q display", m.Key)
		assert.NotEmpty(t, m.Window, "metric %q window", m.Key)
		assert.NotEmpty(t, m.Definition, "metric %q definition", m.Key)
	}

	// The seeded room really is counted, so these are measurements, not zeros.
	assert.GreaterOrEqual(t, hprMetricByKey(t, rooms.Metrics, "agent_messages").Value, 2)
	assert.GreaterOrEqual(t, hprMetricByKey(t, rooms.Metrics, "rooms_with_conversation").Value, 1)
}

func TestHomepageRooms_SelectorDefaultsToTwentyFourHoursAndHonoursEachChoice(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	byDefault, _ := getHomepageRooms(t, ts.URL, "")
	assert.Equal(t, "24h", byDefault.SelectedWindow)
	require.Len(t, byDefault.WindowOptions, 3)
	assert.Equal(t, []string{"24 hours", "7 days", "30 days"}, []string{
		byDefault.WindowOptions[0].Label,
		byDefault.WindowOptions[1].Label,
		byDefault.WindowOptions[2].Label,
	})

	for _, want := range []struct{ value, text string }{
		{"24h", "last 24 hours"},
		{"7d", "last 7 days"},
		{"30d", "last 30 days"},
	} {
		rooms, _ := getHomepageRooms(t, ts.URL, want.value)
		assert.Equal(t, want.value, rooms.SelectedWindow)
		for _, m := range rooms.Metrics {
			assert.Equal(t, want.text, m.Window, "metric %q under window %q", m.Key, want.value)
		}
		for _, o := range rooms.WindowOptions {
			assert.Equal(t, o.Value == want.value, o.Selected, "option %q", o.Value)
		}
	}
}

func TestHomepageRooms_PresenceStaysNowAcrossEveryWindow(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	for _, window := range []string{"24h", "7d", "30d"} {
		rooms, _ := getHomepageRooms(t, ts.URL, window)
		for _, m := range rooms.PresenceMetrics {
			assert.True(t, m.Presence, "%q under %q", m.Key, window)
			assert.Equal(t, "now", m.Window, "%q under %q", m.Key, window)
		}
		assert.Equal(t, "Now", rooms.PresenceHeading)
		assert.NotEmpty(t, rooms.PresenceNote)
	}
}

func TestHomepageRooms_RejectedWindowFallsBackWithoutMislabelling(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	rooms, _ := getHomepageRooms(t, ts.URL, "90d")

	assert.Equal(t, "24h", rooms.SelectedWindow)
	for _, m := range rooms.Metrics {
		assert.Equal(t, "last 24 hours", m.Window,
			"the answer states what it measured, so a fallback can never mislead")
	}
}

func TestHomepageRooms_NeverLeaksAPrivateRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	privateSlug := hpoSlug("privstats")
	secret := "CONFIDENTIAL ROOM STATISTICS CONTENT that must never reach the homepage"
	hpoSeedRoom(t, pool, privateSlug, "Private Stats Room", "hidden", true, []string{secret, secret})

	publicSlug := hpoSlug("pubstats")
	hpoSeedRoom(t, pool, publicSlug, "Public Stats Room", "open", false, []string{"visible", "visible"})

	after, raw := getHomepageRooms(t, ts.URL, "30d")

	// The privacy proof is the body itself: neither the private room's content
	// nor its slug may appear anywhere in a response a logged-out visitor gets.
	assert.NotContains(t, raw, secret)
	assert.NotContains(t, raw, privateSlug)

	// The suite shares one database and other packages delete their own rooms
	// as they finish, so the only stable arithmetic is a floor under rows this
	// test owns. The private room IS counted (a count never names it): that is
	// pinned exactly on a scratch database in
	// TestHomepageStatistics_CountEveryRoomAndShowOnlyPublicOnes, and one layer
	// down in TestGetRoomPulse_CountsPrivateRoomsButNeverDeletedOnes.
	assert.GreaterOrEqual(t, hprMetricByKey(t, after.Metrics, "agent_messages").Value, 2,
		"the public room's two messages were counted")
	assert.GreaterOrEqual(t, hprMetricByKey(t, after.Metrics, "rooms_with_conversation").Value, 1)
}

func TestHomepageRooms_UninstrumentedExchangeMilestoneReadsUnavailable(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	ctx := context.Background()
	// Hide every recorded milestone so the endpoint really has nothing to read.
	_, err := pool.Exec(ctx,
		`UPDATE room_events SET event_type = 'room.activated.hidden.forhprtest' WHERE event_type = $1`,
		db.RoomActivationEventType)
	require.NoError(t, err)
	defer func() {
		pool.Exec(ctx, //nolint:errcheck
			`UPDATE room_events SET event_type = $1 WHERE event_type = 'room.activated.hidden.forhprtest'`,
			db.RoomActivationEventType)
	}()

	rooms, _ := getHomepageRooms(t, ts.URL, "24h")
	exchanges := hprMetricByKey(t, rooms.Metrics, "rooms_with_two_way_exchanges")

	assert.True(t, exchanges.Unavailable)
	assert.NotEqual(t, "0", exchanges.Display, "an unmeasured milestone is never a confident zero")
	assert.NotEmpty(t, exchanges.Qualifier)
}

func TestHomepageRooms_ASecondAgentSpeakingRecordsTheExchangeMilestone(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)

	post := func(agent, content string) {
		t.Helper()
		body := fmt.Sprintf(`{"agent_name":%q,"content":%q}`, agent, content)
		req, err := http.NewRequest("POST", ts.URL+"/r/"+slug+"/message", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+roomToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}

	countMilestones := func() int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(context.Background(), `
			SELECT COUNT(*) FROM room_events e JOIN rooms r ON r.id = e.room_id
			 WHERE r.slug = $1 AND e.event_type = $2
		`, slug, db.RoomActivationEventType).Scan(&n))
		return n
	}

	post("test-planner", "PLANNER ONLINE. Here is the plan.")
	assert.Equal(t, 0, countMilestones(), "one agent talking is not an exchange")

	post("test-executor", "EXECUTOR ONLINE. Plan received.")
	assert.Equal(t, 1, countMilestones(), "the second agent answering activates the room")

	post("test-executor", "IMPLEMENTATION COMPLETE.")
	assert.Equal(t, 1, countMilestones(), "activation is a milestone, not a running count")

	// And the homepage can now say since when it has been measuring.
	rooms, _ := getHomepageRooms(t, ts.URL, "24h")
	exchanges := hprMetricByKey(t, rooms.Metrics, "rooms_with_two_way_exchanges")
	assert.False(t, exchanges.Unavailable)
	assert.GreaterOrEqual(t, exchanges.Value, 1)

	pool.Exec(context.Background(), //nolint:errcheck
		`DELETE FROM room_events WHERE room_id IN (SELECT id FROM rooms WHERE slug = $1)`, slug)
}

func TestHomepageRooms_SparklineBucketsFollowTheWindow(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	for _, want := range []struct {
		window  string
		buckets int
	}{{"24h", 24}, {"7d", 7}, {"30d", 30}} {
		rooms, _ := getHomepageRooms(t, ts.URL, want.window)
		require.NotNil(t, rooms.Sparkline, "window %q", want.window)
		assert.Len(t, rooms.Sparkline.Points, want.buckets, "window %q", want.window)
		for _, p := range rooms.Sparkline.Points {
			assert.NotEmpty(t, p.Label)
			assert.NotEmpty(t, p.Height, "the API decides the bar height, not the browser")
		}
	}
}

func TestHomepageRooms_IsCacheableAndNeedsNoCredentials(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)

	resp, err := http.Get(ts.URL + "/v1/homepage/rooms?window=7d")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "public, no-cache", resp.Header.Get("Cache-Control"), "cacheable, but revalidated so room names never outlive a visibility change")
	_ = time.Now()
}

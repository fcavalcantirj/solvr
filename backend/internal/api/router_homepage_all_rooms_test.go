package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Every room is counted; only public rooms are ever shown (spec.json idx 96).
//
// These tests run the homepage statistics end to end on a SCRATCH database that
// holds exactly one public room and one private room (plus a deleted one), so
// every count can be asserted exactly instead of as a floor over the shared test
// data. They hold both halves of the rule at once:
//
//   - the counted metrics — rooms, messages, milestones, agents online, the
//     time series — include the private room;
//   - every content surface — the activity stream, the room previews, the
//     recently completed rooms, the search terms — shows only the public room,
//     and no response carries anything that identifies the private one.

// hpaRooms is the rooms section as the page reads it.
type hpaRooms struct {
	Heading         string      `json:"heading"`
	Intro           string      `json:"intro"`
	ScopeLabel      string      `json:"scope_label"`
	ScopeNote       string      `json:"scope_note"`
	PresenceMetrics []hpoMetric `json:"presence_metrics"`
	Metrics         []hpoMetric `json:"metrics"`
	Sparkline       *struct {
		Definition string `json:"definition"`
		Points     []struct {
			Value int `json:"value"`
		} `json:"points"`
	} `json:"sparkline"`
	LiveMarker *struct {
		Online bool   `json:"online"`
		Label  string `json:"label"`
	} `json:"live_marker"`
	RecentCompletedRooms []struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"display_name"`
	} `json:"recent_completed_rooms"`
}

func (r hpaRooms) metric(t *testing.T, key string) int {
	t.Helper()
	for _, m := range append(append([]hpoMetric{}, r.PresenceMetrics...), r.Metrics...) {
		if m.Key == key {
			return m.Value
		}
	}
	t.Fatalf("rooms metric %q not found", key)
	return 0
}

func (r hpaRooms) sparkSum() int {
	sum := 0
	if r.Sparkline != nil {
		for _, p := range r.Sparkline.Points {
			sum += p.Value
		}
	}
	return sum
}

// newHomepageScratchURL creates an empty database next to DATABASE_URL's,
// applies every up migration and returns its URL. It is dropped when the test
// ends.
func newHomepageScratchURL(t *testing.T) string {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := pgx.Connect(ctx, base)
	require.NoError(t, err, "connect admin")
	name := fmt.Sprintf("solvr_scratch_hpa_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		admin.Close(ctx)
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 30*time.Second)
		defer cc()
		if _, err := admin.Exec(c, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
		admin.Close(c)
	})

	u, err := url.Parse(base)
	require.NoError(t, err)
	u.Path = "/" + name

	conn, err := pgx.Connect(ctx, u.String())
	require.NoError(t, err, "connect scratch")
	defer conn.Close(ctx)
	files, err := filepath.Glob("../../migrations/*.up.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files, "no up migrations found")
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		require.NoError(t, err)
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
	return u.String()
}

// hpaFixture is the scratch database's whole content, and what must never be
// seen of its private room.
type hpaFixture struct {
	pool *db.Pool

	publicRoom, privateRoom *models.Room
	privateOwner            string
	privateAgents           []string
	privateBodies           []string
	privateEntryIDs         []int64
	privateDescription      string
	privateSearchTerm       string
	publicSearchTerm        string
}

// privateTokens is everything that identifies the private room.
func (f *hpaFixture) privateTokens() []string {
	tokens := []string{
		f.privateRoom.Slug, f.privateRoom.DisplayName, f.privateDescription,
		f.privateRoom.ID.String(), f.privateOwner, f.privateSearchTerm,
	}
	tokens = append(tokens, f.privateAgents...)
	return append(tokens, f.privateBodies...)
}

func hpaExec(t *testing.T, pool *db.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err, "exec %s", sql)
}

// hpaPresence registers an agent, admits it to the room and makes it live.
func hpaPresence(t *testing.T, pool *db.Pool, roomID uuid.UUID, agent string) {
	t.Helper()
	hpaExec(t, pool, `INSERT INTO agents (id, display_name) VALUES ($1, $1) ON CONFLICT (id) DO NOTHING`, agent)
	hpaExec(t, pool, `INSERT INTO room_members (room_id, agent_id, role, added_by)
		VALUES ($1, $2, 'member', 'system') ON CONFLICT (room_id, agent_id) DO NOTHING`, roomID, agent)
	hpaExec(t, pool, `INSERT INTO agent_presence (room_id, agent_id, agent_name, card_json, last_seen, ttl_seconds)
		VALUES ($1, $2, $2, '{}'::jsonb, NOW(), 900)`, roomID, agent)
}

func seedHomepageAllRooms(t *testing.T, pool *db.Pool) *hpaFixture {
	t.Helper()
	ctx := context.Background()
	rooms := db.NewRoomRepository(pool)
	msgs := db.NewMessageRepository(pool)
	events := db.NewRoomEventRepository(pool)

	f := &hpaFixture{
		pool:               pool,
		privateOwner:       uuid.New().String(),
		privateAgents:      []string{"hpa_secret_alpha", "hpa_secret_beta"},
		privateDescription: "hpa confidential purpose nobody outside may read",
		privateSearchTerm:  "hpa private hideout room",
		publicSearchTerm:   "hpa public counted term",
	}

	// The public room: two agents talk, one of them is online.
	f.publicRoom = hpoSeedRoom(t, pool, "test-hpa-public", "HPA Public Counted Room", "a public purpose",
		false, []string{"hpa public message one", "hpa public message two"})
	hpaPresence(t, pool, f.publicRoom.ID, "hpa_public_agent")
	_, err := events.RecordActivation(ctx, f.publicRoom.ID)
	require.NoError(t, err)

	// The private room: a human owner, two private agents online and talking,
	// and the owner's own message.
	hpaExec(t, pool, `INSERT INTO users (id, username, display_name, email, auth_provider, auth_provider_id, role, referral_code)
		VALUES ($1, 'hpaowner', 'HPA Owner', 'hpaowner@test.solvr.dev', 'test', $2, 'user', 'HPAOWNR1')`,
		f.privateOwner, f.privateOwner)
	desc := f.privateDescription
	room, err := rooms.Create(ctx, models.CreateRoomParams{
		Slug: "test-hpa-hideout", DisplayName: "HPA Private Hideout", Description: &desc,
		IsPrivate: true, OwnerID: uuid.MustParse(f.privateOwner),
	})
	require.NoError(t, err)
	f.privateRoom = room
	for i, agent := range f.privateAgents {
		hpaPresence(t, pool, room.ID, agent)
		body := fmt.Sprintf("hpa secret body %d from %s", i, agent)
		m, err := msgs.Create(ctx, models.CreateMessageParams{
			RoomID: room.ID, AuthorType: "agent", AgentName: agent, Content: body, ContentType: "text",
		})
		require.NoError(t, err)
		f.privateBodies = append(f.privateBodies, body)
		f.privateEntryIDs = append(f.privateEntryIDs, m.ID)
	}
	ownerBody := "hpa secret note from the owner"
	owner := f.privateOwner
	m, err := msgs.Create(ctx, models.CreateMessageParams{
		RoomID: room.ID, AuthorType: "human", AuthorID: &owner, AgentName: "human:" + owner,
		Content: ownerBody, ContentType: "text",
	})
	require.NoError(t, err)
	f.privateBodies = append(f.privateBodies, ownerBody)
	f.privateEntryIDs = append(f.privateEntryIDs, m.ID)
	recorded, err := events.RecordActivation(ctx, room.ID)
	require.NoError(t, err)
	require.True(t, recorded, "the private room's two agents are a two-way exchange")

	// Every entry of the private room, events included, is an id that must not leak.
	rows, err := pool.Query(ctx, `SELECT id FROM room_entries WHERE room_id = $1 AND kind = 'event'`, room.ID)
	require.NoError(t, err)
	for rows.Next() {
		var id int64
		require.NoError(t, rows.Scan(&id))
		f.privateEntryIDs = append(f.privateEntryIDs, id)
	}
	rows.Close()
	require.NoError(t, rows.Err())

	// A deleted room, private, with a live agent and a message: gone from everything.
	gone, err := rooms.Create(ctx, models.CreateRoomParams{
		Slug: "test-hpa-deleted", DisplayName: "HPA Deleted Room", IsPrivate: true, OwnerID: uuid.Nil,
	})
	require.NoError(t, err)
	hpaPresence(t, pool, gone.ID, "hpa_deleted_agent")
	_, err = msgs.Create(ctx, models.CreateMessageParams{
		RoomID: gone.ID, AuthorType: "agent", AgentName: "hpa_deleted_agent", Content: "hpa deleted body", ContentType: "text",
	})
	require.NoError(t, err)
	hpaExec(t, pool, `UPDATE rooms SET deleted_at = NOW() WHERE id = $1`, gone.ID)

	// Search terms: one publishable term, and the private room's name searched
	// with a scope that could see private content.
	hpoInsertSearchesAs(t, pool, f.publicSearchTerm, 3, "agent", true, "")
	hpoInsertSearchesAs(t, pool, f.privateSearchTerm, 3, "human", false, "")

	return f
}

// hpaGet calls a public homepage endpoint with no credentials and returns the
// body.
func hpaGet(t *testing.T, baseURL, path string) string {
	t.Helper()
	resp, err := http.Get(baseURL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s: %s", path, string(body))
	return string(body)
}

// assertNoPrivateIdentity walks a whole response: no string anywhere may carry
// the private room's slug, name, description, room id, owner, participants,
// message bodies or search term, and no id-shaped field may carry one of its
// entry ids.
func assertNoPrivateIdentity(t *testing.T, what, body string, f *hpaFixture) {
	t.Helper()
	var doc any
	require.NoError(t, json.Unmarshal([]byte(body), &doc), "%s is JSON", what)

	privateIDs := map[string]bool{}
	for _, id := range f.privateEntryIDs {
		privateIDs[fmt.Sprint(id)] = true
		privateIDs[fmt.Sprintf("message-%d", id)] = true
		privateIDs[fmt.Sprintf("event-%d", id)] = true
	}

	var walk func(key string, v any)
	walk = func(key string, v any) {
		switch val := v.(type) {
		case map[string]any:
			for k, child := range val {
				walk(k, child)
			}
		case []any:
			for _, child := range val {
				walk(key, child)
			}
		case string:
			lower := strings.ToLower(val)
			for _, token := range f.privateTokens() {
				assert.NotContains(t, lower, strings.ToLower(token), "%s: field %q carries a private identity", what, key)
			}
			if key == "id" || strings.HasSuffix(key, "_id") {
				assert.False(t, privateIDs[val], "%s: field %q carries private entry %s", what, key, val)
			}
		case float64:
			if key == "id" || strings.HasSuffix(key, "_id") {
				assert.False(t, privateIDs[fmt.Sprint(int64(val))], "%s: field %q carries a private entry id", what, key)
			}
		}
	}
	walk("", doc)
}

func decodeData(t *testing.T, body string, into any) {
	t.Helper()
	wrapper := struct {
		Data any `json:"data"`
	}{Data: into}
	require.NoError(t, json.Unmarshal([]byte(body), &wrapper))
}

func TestHomepageStatistics_CountEveryRoomAndShowOnlyPublicOnes(t *testing.T) {
	scratch := newHomepageScratchURL(t)
	t.Setenv("DATABASE_URL", scratch)
	// Both rooms are featured: only the public one may appear.

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	f := seedHomepageAllRooms(t, pool)
	featureOnHomepage(t, pool, "test-hpa-hideout", "test-hpa-public")

	endpoints := map[string]string{
		"overview":            hpaGet(t, ts.URL, "/v1/homepage/overview?window=24h"),
		"rooms":               hpaGet(t, ts.URL, "/v1/homepage/rooms?window=24h"),
		"consolidated":        hpaGet(t, ts.URL, "/v1/overview?window=24h"),
		"activity":            hpaGet(t, ts.URL, "/v1/homepage/activity?offset=0&limit=24"),
		"search":              hpaGet(t, ts.URL, "/v1/homepage/search?window=24h"),
		"consolidated 30 day": hpaGet(t, ts.URL, "/v1/overview?window=30d"),
	}
	for what, body := range endpoints {
		assertNoPrivateIdentity(t, what, body, f)
	}

	var ov hpoOverview
	decodeData(t, endpoints["overview"], &ov)
	var overviewRooms struct {
		Rooms hpaRooms `json:"rooms"`
	}
	decodeData(t, endpoints["overview"], &overviewRooms)
	var roomsOnly hpaRooms
	decodeData(t, endpoints["rooms"], &roomsOnly)
	var consolidated struct {
		Rooms     hpaRooms `json:"rooms"`
		Community struct {
			Metrics []hpoMetric `json:"metrics"`
		} `json:"community"`
	}
	decodeData(t, endpoints["consolidated"], &consolidated)

	// COUNTS: exactly the public room plus the private room, never the deleted one.
	for what, rooms := range map[string]hpaRooms{
		"overview": overviewRooms.Rooms, "rooms": roomsOnly, "consolidated": consolidated.Rooms,
	} {
		assert.Equal(t, 3, rooms.metric(t, "agents_online_now"), "%s: 1 public + 2 private agents", what)
		assert.Equal(t, 2, rooms.metric(t, "rooms_with_agents_online_now"), what)
		assert.Equal(t, 2, rooms.metric(t, "rooms_with_conversation"), what)
		assert.Equal(t, 4, rooms.metric(t, "agent_messages"), "%s: 2 public + 2 private", what)
		assert.Equal(t, 1, rooms.metric(t, "human_messages"), "%s: the private owner's note", what)
		assert.Equal(t, 2, rooms.metric(t, "rooms_with_two_way_exchanges"), what)
		assert.Equal(t, 5, rooms.sparkSum(), "%s: every message is a bar", what)
		assert.Equal(t, "All rooms, private ones included", rooms.ScopeLabel, what)
		assert.Contains(t, rooms.ScopeNote, "1 public and 1 private", what)
		assert.NotContains(t, strings.ToLower(rooms.ScopeNote), "never counted", what)
		require.NotNil(t, rooms.LiveMarker, what)
		assert.Equal(t, "3 agents online now, 1 of them in public rooms", rooms.LiveMarker.Label, what)
		assert.Empty(t, rooms.RecentCompletedRooms, "%s: an agent is live in a public room", what)
	}
	for what, metrics := range map[string][]hpoMetric{
		"overview": ov.Community.Metrics, "consolidated": consolidated.Community.Metrics,
	} {
		byKey := map[string]int{}
		for _, m := range metrics {
			byKey[m.Key] = m.Value
		}
		assert.Equal(t, 2, byKey["all_rooms"], "%s: every room that was not deleted", what)
		assert.Equal(t, 1, byKey["public_rooms"], "%s: the public figure beside it", what)
	}

	// CONTENT: only the public room is streamed, previewed or quoted.
	var activity hpoActivity
	decodeData(t, endpoints["activity"], &activity)
	require.NotEmpty(t, activity.entries(), "the public room's messages are streamed")
	for _, stream := range [][]hpoActivityItem{activity.entries(), ov.ActivityRaw.entries()} {
		for _, item := range stream {
			assert.Equal(t, f.publicRoom.Slug, item.RoomSlug, "only the public room is streamed")
		}
	}
	require.Len(t, ov.Previews.Rooms, 1, "the private room is allow-listed but never previewed")
	assert.Equal(t, f.publicRoom.Slug, ov.Previews.Rooms[0].Slug)

	var search hpoSearch
	decodeData(t, endpoints["search"], &search)
	terms := []string{}
	for _, row := range search.Top.Rows {
		terms = append(terms, row.Query)
	}
	assert.Contains(t, terms, f.publicSearchTerm, "the publishable term is listed")
	assert.NotContains(t, terms, f.privateSearchTerm, "a private-scope search term never is")
}

// When the public rooms go quiet, the offline fallback lists recently active
// PUBLIC rooms exactly as before — whether or not agents are working in private
// rooms at that moment — and the 24-hour window still reports private activity
// rather than zero when it is the only activity there is.
func TestHomepageStatistics_PrivateActivityIsCountedButNeverListed(t *testing.T) {
	scratch := newHomepageScratchURL(t)
	t.Setenv("DATABASE_URL", scratch)

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	f := seedHomepageAllRooms(t, pool)
	featureOnHomepage(t, pool, "test-hpa-hideout", "test-hpa-public")

	readRooms := func(window string) (hpaRooms, string) {
		body := hpaGet(t, ts.URL, "/v1/homepage/rooms?window="+window)
		var rooms hpaRooms
		decodeData(t, body, &rooms)
		return rooms, body
	}
	recentSlugs := func(r hpaRooms) []string {
		out := []string{}
		for _, room := range r.RecentCompletedRooms {
			out = append(out, room.Slug)
		}
		return out
	}

	// Nobody online anywhere: the fallback lists the public room only, though
	// the private room was just as active.
	hpaExec(t, pool, `UPDATE agent_presence SET last_seen = NOW() - INTERVAL '2 hours'`)
	quiet, body := readRooms("24h")
	assertNoPrivateIdentity(t, "rooms, nobody online", body, f)
	assert.Equal(t, 0, quiet.metric(t, "agents_online_now"))
	assert.Equal(t, []string{f.publicRoom.Slug}, recentSlugs(quiet))
	require.NotNil(t, quiet.LiveMarker)
	assert.Equal(t, "No agents online now. Recent rooms below.", quiet.LiveMarker.Label)

	// Agents back online in the PRIVATE room only: counted, but the public
	// fallback stays exactly where it was.
	hpaExec(t, pool, `UPDATE agent_presence SET last_seen = NOW() WHERE room_id = $1`, f.privateRoom.ID)
	private, body := readRooms("24h")
	assertNoPrivateIdentity(t, "rooms, private agents online", body, f)
	assert.Equal(t, 2, private.metric(t, "agents_online_now"), "the private agents are online")
	assert.Equal(t, 1, private.metric(t, "rooms_with_agents_online_now"))
	assert.Equal(t, []string{f.publicRoom.Slug}, recentSlugs(private),
		"no agent is in a public room, so the public fallback is unchanged")
	require.NotNil(t, private.LiveMarker)
	assert.True(t, private.LiveMarker.Online)
	assert.Equal(t, "2 agents online now, all in private rooms. Recent public rooms below.", private.LiveMarker.Label)

	// Move the public room's conversation out of the last 24 hours: the private
	// room is then the only activity in the window, and the window reports it.
	hpaExec(t, pool, `UPDATE room_entries SET created_at = NOW() - INTERVAL '3 days' WHERE room_id = $1`, f.publicRoom.ID)
	day, body := readRooms("24h")
	assertNoPrivateIdentity(t, "rooms, only private activity today", body, f)
	assert.Equal(t, 2, day.metric(t, "agent_messages"), "the private room's two agent messages, not zero")
	assert.Equal(t, 1, day.metric(t, "human_messages"))
	assert.Equal(t, 1, day.metric(t, "rooms_with_conversation"))
	assert.Equal(t, 1, day.metric(t, "rooms_with_two_way_exchanges"))
	assert.Equal(t, 3, day.sparkSum(), "the 24-hour series carries the private room's messages")

	week, _ := readRooms("7d")
	assert.Equal(t, 4, week.metric(t, "agent_messages"), "seven days see both rooms again")
	assert.Equal(t, 2, week.metric(t, "rooms_with_conversation"))

	// The stream still shows the public room's (older) messages and nothing of
	// the private room's recent ones.
	body = hpaGet(t, ts.URL, "/v1/homepage/activity?offset=0&limit=24")
	assertNoPrivateIdentity(t, "activity, only private activity today", body, f)
	var activity hpoActivity
	decodeData(t, body, &activity)
	require.NotEmpty(t, activity.entries())
	for _, item := range activity.entries() {
		assert.Equal(t, f.publicRoom.Slug, item.RoomSlug)
	}
}

package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// N-agent collaboration end to end (task: "Verify N-agent collaboration end to end across
// clients and service instances"). Every participant is its own registered agent with its
// own API key and its own per-agent room token; nothing is relayed by a human. Rooms are
// exercised with three, four, eight and a larger fixture of agents so no count, list, slot
// or metric can quietly assume a pair or a maximum of eight.

// nAgentLargeFixture is the "larger configured-capacity" group: rooms have no participant
// cap (capacity_max is optional and unset, the hub runs with no per-room SSE limit), so the
// fixture is simply well past eight.
const nAgentLargeFixture = 20

// nAgentClients are the client products participants identify as in their presence card.
// Solvr has no client-specific code path: each one is just another agent credential.
var nAgentClients = []string{"claude-code", "codex-cli", "openclaw", "hermes"}

type nAgent struct {
	name, client string
	id, key, tok string
}

type nAgentRoom struct {
	slug, roomID, ownerJWT string
	private                bool
	agents                 []*nAgent
	pool                   *db.Pool
}

// nAgentDB opens a pool for assertions made straight against the database.
func nAgentDB(t *testing.T) *db.Pool {
	t.Helper()
	pool, err := db.NewPool(context.Background(), os.Getenv("DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// nAgentName is the role of participant i of n: a planner, executors, and (from three
// agents up) a reviewer.
func nAgentName(i, n int) string {
	switch {
	case i == 0:
		return "planner"
	case i == n-1 && n >= 3:
		return "reviewer"
	default:
		return "executor-" + strconv.Itoa(i)
	}
}

// newNAgentRoom creates a room owned by a human and admits n distinct agents through inst:
// each registers, is added by the owner when the room is private, handshakes for its own
// room token and joins presence with a card naming its client product.
func newNAgentRoom(t *testing.T, inst *roomInstance, pool *db.Pool, n int, private bool) *nAgentRoom {
	t.Helper()
	_, ownerJWT := createRoomTestUser(t, pool)
	room := &nAgentRoom{slug: entriesTestRoom(t, inst.ts.URL, ownerJWT, private), ownerJWT: ownerJWT, private: private, pool: pool}
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT id::text FROM rooms WHERE slug=$1`, room.slug).Scan(&room.roomID))
	for i := 0; i < n; i++ {
		room.agents = append(room.agents, room.admit(t, inst, nAgentName(i, n), nAgentClients[i%len(nAgentClients)]))
	}
	return room
}

func (r *nAgentRoom) admit(t *testing.T, inst *roomInstance, name, client string) *nAgent {
	t.Helper()
	a := &nAgent{name: name, client: client}
	a.id, a.key = registerRoomTestAgent(t, inst.ts)
	if r.private {
		status, out := doJSON(t, http.MethodPost, inst.ts.URL+"/v1/rooms/"+r.slug+"/members", r.ownerJWT, `{"agent_id":"`+a.id+`"}`)
		require.Equal(t, http.StatusCreated, status, "admit %s: %v", name, out)
	}
	a.tok = handshakeRoomToken(t, inst.ts, r.slug, a.key)
	r.join(t, inst, a)
	return a
}

// join (re)announces a's presence with a card naming its client product.
func (r *nAgentRoom) join(t *testing.T, inst *roomInstance, a *nAgent) {
	t.Helper()
	status, out := doJSON(t, http.MethodPost, inst.ts.URL+"/r/"+r.slug+"/join", a.tok, fmt.Sprintf(
		`{"agent_name":%q,"ttl_seconds":600,"card":{"name":%q,"description":"n-agent participant","client":%q,"skills":[]}}`,
		a.name, a.name, a.client))
	require.Equal(t, http.StatusOK, status, "join %s: %v", a.name, out)
}

// postEntryRaw writes a message through the canonical POST /v1/rooms/{slug}/entries. It
// never calls t.FailNow, so concurrent writers can use it from goroutines.
func postEntryRaw(base, slug, bearer string, payload map[string]any) (int, int64, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return 0, 0, err
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/rooms/"+slug+"/entries", bytes.NewReader(raw))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return resp.StatusCode, 0, fmt.Errorf("decode %s: %w", body, err)
	}
	return resp.StatusCode, out.Data.ID, nil
}

// post writes a message as a (with its own room token) and returns the new entry id.
// replyTo 0 means "not a reply".
func (r *nAgentRoom) post(t *testing.T, base string, a *nAgent, body string, replyTo int64) int64 {
	t.Helper()
	payload := map[string]any{"body": body}
	if replyTo != 0 {
		payload["reply_to_entry_id"] = replyTo
	}
	status, id, err := postEntryRaw(base, r.slug, a.tok, payload)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status, "%s posts %q", a.name, body)
	require.NotZero(t, id)
	return id
}

// messages returns the room's message entries in timeline order.
func (r *nAgentRoom) messages(t *testing.T, base, bearer string) []map[string]any {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, base+"/v1/rooms/"+r.slug+"/entries?kind=message&limit=100", bearer, "")
	require.Equal(t, http.StatusOK, status, "list messages: %v", out)
	var res []map[string]any
	for _, raw := range out["data"].([]any) {
		res = append(res, raw.(map[string]any))
	}
	return res
}

// streams opens one live stream per agent, each with that agent's own room token.
func (r *nAgentRoom) streams(t *testing.T, base string) []*liveStream {
	t.Helper()
	out := make([]*liveStream, len(r.agents))
	for i, a := range r.agents {
		out[i] = openLiveStream(t, base+"/v1/rooms/"+r.slug+"/stream", a.tok)
	}
	return out
}

// countDB runs a COUNT query with the room id as $1.
func (r *nAgentRoom) countDB(t *testing.T, query string) int {
	t.Helper()
	var n int
	require.NoError(t, r.pool.QueryRow(context.Background(), query, r.roomID).Scan(&n))
	return n
}

func (r *nAgentRoom) activations(t *testing.T) int {
	t.Helper()
	return r.countDB(t, `SELECT COUNT(*) FROM room_events WHERE room_id=$1::uuid AND event_type='`+db.RoomActivationEventType+`'`)
}

func (r *nAgentRoom) funnelSteps(t *testing.T, step string) int {
	t.Helper()
	return r.countDB(t, `SELECT COUNT(*) FROM funnel_events WHERE room_id=$1::uuid AND event_name='`+step+`'`)
}

// --- Steps 1-2: planner, two executors and a reviewer finish a scoped exchange. ---

func TestNAgentRoom_PlannerTwoExecutorsAndReviewerCompleteAScopedExchange(t *testing.T) {
	a := startRoomInstance(t, RoomRelayOptions{})
	pool := nAgentDB(t)
	roomPreCleanup(t, pool)
	room := newNAgentRoom(t, a, pool, 4, false)
	planner, exec1, exec2, reviewer := room.agents[0], room.agents[1], room.agents[2], room.agents[3]
	streams := room.streams(t, a.ts.URL)

	// Individual credentials: four distinct agents, four distinct per-agent room tokens.
	seen := map[string]bool{}
	for _, ag := range room.agents {
		require.NotContains(t, seen, ag.tok, "every participant has its own room token")
		seen[ag.tok] = true
	}

	// The planner uses the A2A transport adapter, everyone else the canonical entries API.
	plan := postRoomMessage(t, a.ts.URL, room.slug, planner.tok, planner.name, "plan: executor-1 builds the API, executor-2 builds the UI")
	build1 := room.post(t, a.ts.URL, exec1, "build: API endpoint done", plan)
	build2 := room.post(t, a.ts.URL, exec2, "build: UI page done", plan)
	review1 := room.post(t, a.ts.URL, reviewer, "review: API approved", build1)
	review2 := room.post(t, a.ts.URL, reviewer, "review: UI needs an aria label", build2)
	fix := room.post(t, a.ts.URL, exec2, "build: aria label added", review2)
	review3 := room.post(t, a.ts.URL, reviewer, "review: UI approved", fix)
	done := room.post(t, a.ts.URL, planner, "plan complete: both parts reviewed", review3)

	type want struct {
		id      int64
		author  *nAgent
		replyTo int64
	}
	script := []want{{plan, planner, 0}, {build1, exec1, plan}, {build2, exec2, plan}, {review1, reviewer, build1},
		{review2, reviewer, build2}, {fix, exec2, review2}, {review3, reviewer, fix}, {done, planner, review3}}
	msgs := room.messages(t, a.ts.URL, reviewer.tok)
	require.Len(t, msgs, len(script), "the exchange is exactly the eight agent messages, no human relay")
	for i, w := range script {
		m := msgs[i]
		require.Equal(t, float64(w.id), m["id"], "entry %d is in submission order", i)
		require.Equal(t, "agent", m["author_type"], "entry %d written by an agent, not relayed by a human", i)
		require.Equal(t, w.author.id, m["author_id"], "entry %d is attributed to %s's own identity", i, w.author.name)
		if w.replyTo == 0 {
			require.Nil(t, m["reply_to_entry_id"])
		} else {
			require.Equal(t, float64(w.replyTo), m["reply_to_entry_id"], "entry %d answers the right entry", i)
		}
	}

	// All four joined the same room, each announcing its own client product.
	status, out := doJSON(t, http.MethodGet, a.ts.URL+"/r/"+room.slug+"/agents", exec1.tok, "")
	require.Equal(t, http.StatusOK, status)
	clients := map[string]string{}
	for _, raw := range out["data"].([]any) {
		rec := raw.(map[string]any)
		var card map[string]any
		cardRaw, _ := json.Marshal(rec["card_json"])
		require.NoError(t, json.Unmarshal(cardRaw, &card))
		clients[rec["agent_name"].(string)] = card["client"].(string)
	}
	require.Equal(t, map[string]string{"planner": "claude-code", "executor-1": "codex-cli", "executor-2": "openclaw", "reviewer": "hermes"}, clients)

	// Every participant's stream received the same committed timeline, in order, once.
	committed := entriesAfter(t, a.ts.URL, room.slug, planner.key, 0, "")
	require.Subset(t, committed, []int64{plan, build1, build2, review1, review2, fix, review3, done})
	for i, s := range streams {
		require.Equal(t, committed, s.waitIDs(t, len(committed)), "%s's stream", room.agents[i].name)
	}
}

// --- Step 3: three, eight and a larger fixture: nothing assumes a pair or eight. ---

func TestNAgentRoom_GroupSizesThreeEightAndLargerHaveNoPairOrEightCap(t *testing.T) {
	for _, n := range []int{3, 8, nAgentLargeFixture} {
		t.Run(strconv.Itoa(n)+"_agents", func(t *testing.T) {
			a := startRoomInstance(t, RoomRelayOptions{})
			pool := nAgentDB(t)
			roomPreCleanup(t, pool)
			room := newNAgentRoom(t, a, pool, n, false)
			streams := room.streams(t, a.ts.URL)

			plan := room.post(t, a.ts.URL, room.agents[0], "plan for "+strconv.Itoa(n), 0)
			for _, ag := range room.agents[1:] {
				room.post(t, a.ts.URL, ag, ag.name+" reporting", plan)
			}

			// Canonical timeline: n messages from n distinct identities.
			authors := map[any]bool{}
			for _, m := range room.messages(t, a.ts.URL, room.agents[0].tok) {
				authors[m["author_id"]] = true
			}
			require.Len(t, authors, n, "every participant's message is attributed to its own identity")

			// Presence and room detail list every participant.
			require.Len(t, presentNames(t, a, room.slug, room.agents[n-1].tok), n)
			status, out := doJSON(t, http.MethodGet, a.ts.URL+"/v1/rooms/"+room.slug, "", "")
			require.Equal(t, http.StatusOK, status)
			detail := entryData(t, out)
			require.Equal(t, float64(n), detail["online_count"])
			require.Len(t, detail["agents"], n)
			require.Equal(t, "conversation_started", detail["connection_status"])

			// Metrics: n distinct joins with ordinals 1..n, and exactly one activation.
			require.Equal(t, n, room.funnelSteps(t, "participant_joined"))
			require.Equal(t, n, room.countDB(t, `SELECT MAX(ordinal) FROM funnel_events WHERE room_id=$1::uuid AND event_name='participant_joined'`))
			require.Equal(t, 1, room.funnelSteps(t, "first_two_way_exchange"))
			require.Equal(t, 1, room.activations(t))

			// Public discovery card: n distinct participants, n live agents, n messages.
			card := publicRoomCard(t, a.ts.URL, room.slug)
			require.Equal(t, float64(n), card["unique_participant_count"])
			require.Equal(t, float64(n), card["live_agent_count"])
			require.Equal(t, float64(n), card["message_count"])

			// Homepage preview of this room: bounded names, but the true participant count.
			preview := homepagePreview(t, room.slug)
			require.Equal(t, float64(n), preview["participant_count"], "the preview states the real participant count")
			shown := len(preview["participants"].([]any))
			if shown < n {
				require.Equal(t, fmt.Sprintf("+%d more participants", n-shown), preview["more_participants_label"])
			} else {
				require.Empty(t, preview["more_participants_label"])
			}

			committed := entriesAfter(t, a.ts.URL, room.slug, room.agents[0].key, 0, "")
			for i, s := range streams {
				require.Equal(t, committed, s.waitIDs(t, len(committed)), "%s's stream", room.agents[i].name)
			}
		})
	}
}

// publicRoomCard finds slug in the anonymous public room list and returns its card.
func publicRoomCard(t *testing.T, base, slug string) map[string]any {
	t.Helper()
	status, out := doJSON(t, http.MethodGet, base+"/v1/rooms?q="+slug, "", "")
	require.Equal(t, http.StatusOK, status, "public room list: %v", out)
	for _, raw := range out["data"].([]any) {
		if card := raw.(map[string]any); card["slug"] == slug {
			return card
		}
	}
	t.Fatalf("room %s missing from the public list: %v", slug, out)
	return nil
}

// homepagePreview reads the homepage overview from a fresh instance (empty 30s snapshot)
// with slug as the editorially selected preview room, and returns that preview.
func homepagePreview(t *testing.T, slug string) map[string]any {
	t.Helper()
	previews, _ := homepagePreviews(t, slug)
	require.Len(t, previews, 1, "the selected public room is previewed")
	return previews[0]
}

// homepagePreviews reads the homepage overview from a fresh instance with slugs (comma
// separated) as the editorial allow-list, returning the previews and the raw body.
func homepagePreviews(t *testing.T, slugs string) ([]map[string]any, string) {
	t.Helper()
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", slugs)
	fresh := startRoomInstance(t, RoomRelayOptions{})
	resp, err := http.Get(fresh.ts.URL + "/v1/homepage/overview")
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "overview: %s", body)
	var out struct {
		Data struct {
			Previews struct {
				Rooms []map[string]any `json:"rooms"`
			} `json:"previews"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &out))
	return out.Data.Previews.Rooms, string(body)
}

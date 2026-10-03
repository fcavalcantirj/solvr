package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// "Try this workflow" / "Start a new room" (idx 88 steps 1-2, idx 92 fresh-room reuse):
// a fresh room may name a PUBLIC source room. The API copies only the selected task
// structure (description, category, tags) and records the source; it never copies
// memberships, credentials, pins, events or results.

func createSourceTestRoom(t *testing.T, ts *httptest.Server, bearer, body string) (int, map[string]interface{}) {
	t.Helper()
	resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms", body, bearer)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &env), "body: %s", raw)
	return resp.StatusCode, env
}

func sourceTestSlug(tag string) string {
	return fmt.Sprintf("test-src-%s-%d", tag, time.Now().UnixNano()%1000000000)
}

func TestCreateRoom_FromAPublicSourceRoomCopiesOnlyTheTaskStructure(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	srcSlug := sourceTestSlug("origin")
	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(
		`{"display_name":"Origin","slug":%q,"description":"two agents build a game","category":"games","tags":["game","ttt"]}`, srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	srcID := env["data"].(map[string]interface{})["id"].(string)

	// The source room has a member agent, a message and a pin: none of it may travel.
	token := admitRoomAgent(t, ts, srcSlug, "")
	post := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+srcSlug+"/entries",
		`{"body":"Build tic-tac-toe","client_entry_id":"src-1"}`, token)
	raw, _ := io.ReadAll(post.Body)
	post.Body.Close()
	require.Equal(t, http.StatusCreated, post.StatusCode, "%s", raw)
	_, err := pool.Exec(context.Background(),
		`UPDATE messages SET pinned_at = NOW() WHERE room_id = $1::uuid`, srcID)
	require.NoError(t, err)

	_, agentKey := registerRoomTestAgent(t, ts)
	freshSlug := sourceTestSlug("fresh")
	code, env = createSourceTestRoom(t, ts, agentKey, fmt.Sprintf(
		`{"display_name":"My version","slug":%q,"source_room":%q}`, freshSlug, srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	data := env["data"].(map[string]interface{})
	require.Equal(t, srcID, data["source_room_id"])
	require.Equal(t, "two agents build a game", data["description"])
	require.Equal(t, "games", data["category"])
	require.Equal(t, []interface{}{"game", "ttt"}, data["tags"])
	freshID := data["id"].(string)

	var members, entries, pins int
	ctx := context.Background()
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_members WHERE room_id = $1::uuid AND revoked_at IS NULL`, freshID).Scan(&members))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_entries WHERE room_id = $1::uuid`, freshID).Scan(&entries))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_entries WHERE room_id = $1::uuid AND pinned_at IS NOT NULL`, freshID).Scan(&pins))
	require.Equal(t, 1, members, "only the creator owns the fresh room")
	require.Equal(t, 0, entries, "no message or event of the source travels")
	require.Equal(t, 0, pins)
}

func TestCreateRoom_ExplicitFieldsWinOverTheSourceRoom(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	srcSlug := sourceTestSlug("origin2")
	code, _ := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(
		`{"display_name":"Origin","slug":%q,"description":"theirs","tags":["a"]}`, srcSlug))
	require.Equal(t, http.StatusCreated, code)

	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(
		`{"display_name":"Mine","slug":%q,"source_room":%q,"description":"mine","tags":["b"]}`, sourceTestSlug("mine"), srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	data := env["data"].(map[string]interface{})
	require.Equal(t, "mine", data["description"])
	require.Equal(t, []interface{}{"b"}, data["tags"])
}

func TestCreateRoom_APrivateOrMissingSourceRoomIsRefusedAlike(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	privSlug := sourceTestSlug("priv")
	code, _ := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Hidden","slug":%q,"is_private":true}`, privSlug))
	require.Equal(t, http.StatusCreated, code)

	var messages []string
	for _, src := range []string{privSlug, sourceTestSlug("missing")} {
		code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(
			`{"display_name":"Copy","slug":%q,"source_room":%q}`, sourceTestSlug("copy"), src))
		require.Equal(t, http.StatusBadRequest, code, "%v", env)
		e := env["error"].(map[string]interface{})
		require.Equal(t, "INVALID_SOURCE_ROOM", e["code"])
		messages = append(messages, e["message"].(string))
	}
	require.Equal(t, messages[0], messages[1], "the answer must not reveal whether a private room exists")

	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM rooms WHERE display_name = 'Copy' AND slug LIKE 'test-src-copy-%'`).Scan(&n))
	require.Zero(t, n, "a refused create stores nothing")
}

// The whole reuse chain through the real router: a finished public room offers its task
// to a fresh start, and the fresh room keeps none of the old room's state.
func TestTryThisWorkflow_AFinishedPublicRoomSeedsAFreshRoomEndToEnd(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	srcSlug := sourceTestSlug("done")
	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Finished build","slug":%q,"tags":["ttt"]}`, srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	token := admitRoomAgent(t, ts, srcSlug, "")
	post := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+srcSlug+"/entries",
		`{"body":"Build tic-tac-toe; my key is solvr_rt_abcdefghijklmnopqrstuvwx","client_entry_id":"done-1"}`, token)
	post.Body.Close()
	require.Equal(t, http.StatusCreated, post.StatusCode)
	arch := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+srcSlug+"/archive", `{}`, jwt)
	arch.Body.Close()
	require.Equal(t, http.StatusOK, arch.StatusCode)

	resp, err := http.Get(ts.URL + "/v1/connect?from_room=" + srcSlug)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
	var start struct {
		Data struct {
			Selected struct {
				Task       string `json:"task"`
				SourceRoom string `json:"source_room"`
			} `json:"selected"`
			Source *struct {
				Kind     string `json:"kind"`
				RoomSlug string `json:"room_slug"`
			} `json:"source"`
			Prompt struct {
				Text string `json:"text"`
			} `json:"prompt"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &start))
	require.NotNil(t, start.Data.Source)
	require.Equal(t, "room", start.Data.Source.Kind)
	require.Equal(t, srcSlug, start.Data.Selected.SourceRoom)
	require.Contains(t, start.Data.Selected.Task, "Build tic-tac-toe")
	require.NotContains(t, string(raw), "solvr_rt_abcdefghijklmnopqrstuvwx")
	require.Contains(t, start.Data.Prompt.Text, fmt.Sprintf(`"source_room": %q`, srcSlug))

	_, agentKey := registerRoomTestAgent(t, ts)
	code, env = createSourceTestRoom(t, ts, agentKey, fmt.Sprintf(
		`{"display_name":"Again","slug":%q,"source_room":%q}`, sourceTestSlug("again"), srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	data := env["data"].(map[string]interface{})
	require.NotNil(t, data["source_room_id"])
	require.Nil(t, data["archived_at"], "the fresh room is open, not finished")
	require.Nil(t, data["result_message_id"])
}

// The room_created funnel step carries the validated source, so attribution survives from
// the incoming link through creation into activation (idx 88 step 4).
func TestCreateRoom_TheRoomCreatedStepIsAttributedToItsSource(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	srcSlug := sourceTestSlug("attr")
	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Attr origin","slug":%q}`, srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	srcID := env["data"].(map[string]interface{})["id"].(string)

	_, agentKey := registerRoomTestAgent(t, ts)
	code, env = createSourceTestRoom(t, ts, agentKey, fmt.Sprintf(
		`{"display_name":"Attr fresh","slug":%q,"source_room":%q,"flow_id":"f_attr_test"}`, sourceTestSlug("attrfresh"), srcSlug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	freshID := env["data"].(map[string]interface{})["id"].(string)

	var kind, id, flow string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT COALESCE(source_kind, ''), COALESCE(source_id::text, ''), COALESCE(flow_id, '')
		  FROM funnel_events WHERE room_id = $1::uuid AND event_name = 'room_created'`, freshID).Scan(&kind, &id, &flow))
	require.Equal(t, "room", kind)
	require.Equal(t, srcID, id)
	require.Equal(t, "f_attr_test", flow)

	var srcKind string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT COALESCE(source_kind, '') FROM funnel_events WHERE room_id = $1::uuid AND event_name = 'room_created'`,
		srcID).Scan(&srcKind))
	require.Empty(t, srcKind, "a room created without a source stays unattributed")
}

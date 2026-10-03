package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// GET /v1/rooms/{slug}/share (idx 88 step 3): a clean room link, a share link, a "try
// this workflow" link and an optional outcome excerpt the human may copy. Solvr never
// posts it anywhere, and a private room is never excerpted.

type roomShareResponse struct {
	Data struct {
		RoomURL  string `json:"room_url"`
		ShareURL string `json:"share_url"`
		TryURL   string `json:"try_url"`
		Excerpt  struct {
			Title  string `json:"title"`
			Text   string `json:"text"`
			Source string `json:"source"`
		} `json:"excerpt"`
		CopyText string `json:"copy_text"`
		Note     string `json:"note"`
	} `json:"data"`
}

func getRoomShare(t *testing.T, url, bearer string) (int, roomShareResponse, string) {
	t.Helper()
	resp := doRoomRequest(t, "GET", url, "", bearer)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out roomShareResponse
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &out), "%s", raw)
	}
	return resp.StatusCode, out, string(raw)
}

func TestRoomShare_APublicRoomOffersCleanLinksAndAScrubbedExcerpt(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	slug := sourceTestSlug("share")
	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Share me","slug":%q}`, slug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	token := admitRoomAgent(t, ts, slug, "")
	post := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries",
		`{"body":"Build the scoreboard. Token solvr_rt_abcdefghijklmnopqrstuvwx stays secret.","client_entry_id":"sh-1"}`, token)
	post.Body.Close()
	require.Equal(t, http.StatusCreated, post.StatusCode)

	code, out, raw := getRoomShare(t, ts.URL+"/v1/rooms/"+slug+"/share", "")
	require.Equal(t, http.StatusOK, code, raw)
	require.Equal(t, "https://solvr.dev/rooms/"+slug, out.Data.RoomURL)
	require.Equal(t, "https://solvr.dev/rooms/"+slug+"?via=share", out.Data.ShareURL)
	require.Equal(t, "https://solvr.dev/connect?from_room="+slug, out.Data.TryURL)
	require.Equal(t, "initial_task", out.Data.Excerpt.Source)
	require.Equal(t, "Share me", out.Data.Excerpt.Title)
	require.Contains(t, out.Data.Excerpt.Text, "Build the scoreboard.")
	require.NotContains(t, raw, "solvr_rt_abcdefghijklmnopqrstuvwx")
	require.LessOrEqual(t, len([]rune(out.Data.Excerpt.Text)), 280)
	require.Contains(t, out.Data.CopyText, out.Data.ShareURL)
	require.Contains(t, out.Data.CopyText, "Build the scoreboard.")
	require.Contains(t, out.Data.Note, "never posts")
}

func TestRoomShare_ExcerptPrefersTheResultThenThePinOverTheInitialTask(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)
	ctx := context.Background()

	slug := sourceTestSlug("prio")
	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Prio","slug":%q}`, slug))
	require.Equal(t, http.StatusCreated, code, "%v", env)
	roomID := env["data"].(map[string]interface{})["id"].(string)
	token := admitRoomAgent(t, ts, slug, "")
	for i, body := range []string{"the task", "the directive", "the result"} {
		resp := doRoomRequest(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries",
			fmt.Sprintf(`{"body":%q,"client_entry_id":"prio-%d"}`, body, i), token)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode)
	}
	_, err := pool.Exec(ctx, `UPDATE messages SET pinned_at = NOW() WHERE room_id = $1::uuid AND content = 'the directive'`, roomID)
	require.NoError(t, err)

	_, out, raw := getRoomShare(t, ts.URL+"/v1/rooms/"+slug+"/share", "")
	require.Equal(t, "pinned", out.Data.Excerpt.Source, raw)
	require.Equal(t, "the directive", out.Data.Excerpt.Text)

	_, err = pool.Exec(ctx, `UPDATE rooms SET result_message_id = (SELECT id FROM messages WHERE room_id = $1::uuid AND content = 'the result')
		WHERE id = $1::uuid`, roomID)
	require.NoError(t, err)
	_, out, raw = getRoomShare(t, ts.URL+"/v1/rooms/"+slug+"/share", "")
	require.Equal(t, "result", out.Data.Excerpt.Source, raw)
	require.Equal(t, "the result", out.Data.Excerpt.Text)
}

func TestRoomShare_APrivateRoomIsNeverExcerpted(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	slug := sourceTestSlug("privshare")
	code, env := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Hidden","slug":%q,"is_private":true}`, slug))
	require.Equal(t, http.StatusCreated, code, "%v", env)

	code, _, raw := getRoomShare(t, ts.URL+"/v1/rooms/"+slug+"/share", jwt)
	require.Equal(t, http.StatusConflict, code, raw)
	require.Contains(t, raw, "ROOM_PRIVATE")

	code, _, _ = getRoomShare(t, ts.URL+"/v1/rooms/"+slug+"/share", "")
	require.Contains(t, []int{http.StatusForbidden, http.StatusNotFound, http.StatusUnauthorized}, code,
		"an outsider is stopped by the room policy before anything is read")
}

func TestGetRoom_OffersTryThisWorkflowOnlyForPublicRooms(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	_, jwt := createRoomTestUser(t, pool)

	pub := sourceTestSlug("trypub")
	code, _ := createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Pub","slug":%q}`, pub))
	require.Equal(t, http.StatusCreated, code)
	priv := sourceTestSlug("trypriv")
	code, _ = createSourceTestRoom(t, ts, jwt, fmt.Sprintf(`{"display_name":"Priv","slug":%q,"is_private":true}`, priv))
	require.Equal(t, http.StatusCreated, code)

	for slug, want := range map[string]interface{}{pub: "/connect?from_room=" + pub, priv: nil} {
		resp := doRoomRequest(t, "GET", ts.URL+"/v1/rooms/"+slug, "", jwt)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
		var env map[string]map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &env))
		got, present := env["data"]["try_workflow_url"]
		require.True(t, present, "the field is always present (null when not offered)")
		require.Equal(t, want, got, slug)
	}
}

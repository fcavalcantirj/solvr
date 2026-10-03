package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// Opt-in room notifications through the real router (idx 92 step 3): a person opts in per
// room, is told about actual replies to them and requested reviews, can turn a room off or
// pause everything, and is never told about agent heartbeats, joins or pins.

type roomNotificationsState struct {
	Subscribed bool     `json:"subscribed"`
	Paused     bool     `json:"paused"`
	Events     []string `json:"events"`
	Off        string   `json:"off"`
}

func roomNotifications(t *testing.T, f pinFixture, method, bearer string) (int, roomNotificationsState) {
	t.Helper()
	resp := doRoomRequest(t, method, f.ts.URL+"/v1/rooms/"+f.slug+"/notifications", "", bearer)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var env struct {
		Data roomNotificationsState `json:"data"`
	}
	if resp.StatusCode == http.StatusOK {
		require.NoError(t, json.Unmarshal(raw, &env), "%s", raw)
	}
	return resp.StatusCode, env.Data
}

type listedNotification struct {
	Type          string `json:"type"`
	SchemaVersion int    `json:"schema_version"`
	Subject       struct {
		RoomID  string `json:"room_id"`
		EntryID int64  `json:"entry_id"`
	} `json:"subject"`
	Body string `json:"body"`
}

func myNotifications(t *testing.T, f pinFixture, bearer string) []listedNotification {
	t.Helper()
	resp := doRoomRequest(t, "GET", f.ts.URL+"/v1/notifications?per_page=50", "", bearer)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
	var env struct {
		Data []listedNotification `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env), "%s", raw)
	var room []listedNotification
	for _, n := range env.Data {
		if n.Subject.RoomID == f.roomID {
			room = append(room, n)
		}
	}
	return room
}

func (f pinFixture) humanPost(t *testing.T, body string) int64 {
	t.Helper()
	resp := doRoomRequest(t, "POST", f.ts.URL+"/v1/rooms/"+f.slug+"/entries", body, f.owner)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", raw)
	var env struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &env))
	return env.Data.ID
}

func setRoomNotificationSettings(t *testing.T, f pinFixture, bearer, value string) {
	t.Helper()
	resp := doRoomRequest(t, "PATCH", f.ts.URL+"/v1/me/notification-settings", fmt.Sprintf(`{"room_notifications":%q}`, value), bearer)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
	require.Contains(t, string(raw), fmt.Sprintf(`"room_notifications":%q`, value))
}

func TestRoomNotifications_OffByDefaultThenASubscribedHumanIsToldAboutAReply(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()

	code, st := roomNotifications(t, f, "GET", f.owner)
	require.Equal(t, http.StatusOK, code)
	require.False(t, st.Subscribed, "opt-in: off by default")
	require.Equal(t, []string{"room.reply", "room.review_requested"}, st.Events)
	require.Equal(t, "DELETE /v1/rooms/"+f.slug+"/notifications", st.Off)

	mine := f.humanPost(t, `{"body":"Can someone check the scoreboard?","client_entry_id":"n-1"}`)
	f.post(t, fmt.Sprintf(`{"body":"before opting in","client_entry_id":"n-2","reply_to_entry_id":%d}`, mine))
	require.Empty(t, myNotifications(t, f, f.owner), "nothing before opting in")

	code, st = roomNotifications(t, f, "PUT", f.owner)
	require.Equal(t, http.StatusOK, code)
	require.True(t, st.Subscribed)
	reply := f.post(t, fmt.Sprintf(`{"body":"Checked: it works","client_entry_id":"n-3","reply_to_entry_id":%d}`, mine))

	got := myNotifications(t, f, f.owner)
	require.Len(t, got, 1)
	require.Equal(t, "room.reply", got[0].Type)
	require.Equal(t, 3, got[0].SchemaVersion)
	require.Equal(t, reply, got[0].Subject.EntryID)
	require.NotContains(t, got[0].Body, "Checked: it works")
}

func TestRoomNotifications_HeartbeatsJoinsAndPinsNeverNotify(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	code, _ := roomNotifications(t, f, "PUT", f.owner)
	require.Equal(t, http.StatusOK, code)
	entry := f.humanPost(t, `{"body":"the directive","client_entry_id":"hb-1"}`)

	for _, call := range []struct{ method, path, body string }{
		{"POST", "/r/" + f.slug + "/join", `{"agent_name":"heartbeat_bot"}`},
		{"POST", "/r/" + f.slug + "/heartbeat", `{"agent_name":"heartbeat_bot"}`},
		{"POST", "/r/" + f.slug + "/heartbeat", `{"agent_name":"heartbeat_bot"}`},
		{"POST", fmt.Sprintf("/v1/rooms/%s/entries/%d/pin", f.slug, entry), ""},
	} {
		resp := doRoomRequest(t, call.method, f.ts.URL+call.path, call.body, f.token)
		resp.Body.Close()
		require.Less(t, resp.StatusCode, 300, "%s %s", call.method, call.path)
	}
	require.Empty(t, myNotifications(t, f, f.owner), "presence and pins are not replies")
}

func TestRoomNotifications_PerRoomOffAndTheGlobalPause(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	mine := f.humanPost(t, `{"body":"question","client_entry_id":"off-1"}`)

	code, _ := roomNotifications(t, f, "PUT", f.owner)
	require.Equal(t, http.StatusOK, code)
	code, st := roomNotifications(t, f, "DELETE", f.owner)
	require.Equal(t, http.StatusOK, code)
	require.False(t, st.Subscribed)
	f.post(t, fmt.Sprintf(`{"body":"a","client_entry_id":"off-2","reply_to_entry_id":%d}`, mine))
	require.Empty(t, myNotifications(t, f, f.owner), "the room is off")

	roomNotifications(t, f, "PUT", f.owner)
	setRoomNotificationSettings(t, f, f.owner, "paused")
	_, st = roomNotifications(t, f, "GET", f.owner)
	require.True(t, st.Subscribed)
	require.True(t, st.Paused)
	f.post(t, fmt.Sprintf(`{"body":"b","client_entry_id":"off-3","reply_to_entry_id":%d}`, mine))
	require.Empty(t, myNotifications(t, f, f.owner), "paused everywhere")

	setRoomNotificationSettings(t, f, f.owner, "on")
	f.post(t, fmt.Sprintf(`{"body":"c","client_entry_id":"off-4","reply_to_entry_id":%d}`, mine))
	require.Len(t, myNotifications(t, f, f.owner), 1)
}

func TestRoomNotifications_AReviewRequestReachesSubscribers(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	roomNotifications(t, f, "PUT", f.owner)

	resp := doRoomRequest(t, "POST", f.ts.URL+"/v1/rooms/"+f.slug+"/entries",
		`{"kind":"event","event_type":"review.requested","issue":"#1","client_entry_id":"rv-1"}`, f.token)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "%s", raw)

	got := myNotifications(t, f, f.owner)
	require.Len(t, got, 1)
	require.Equal(t, "room.review_requested", got[0].Type)
}

func TestRoomNotifications_AnonymousCannotOptInAndSettingsNeedAnAccount(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	code, _ := roomNotifications(t, f, "PUT", "")
	require.Equal(t, http.StatusUnauthorized, code)
	resp := doRoomRequest(t, "GET", f.ts.URL+"/v1/me/notification-settings", "", "")
	resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp = doRoomRequest(t, "PATCH", f.ts.URL+"/v1/me/notification-settings", `{"room_notifications":"maybe"}`, f.owner)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRoomViewer_ReportsTheNotificationState(t *testing.T) {
	f := newPinFixture(t)
	defer f.cleanup()
	roomNotifications(t, f, "PUT", f.owner)

	resp := doRoomRequest(t, "GET", f.ts.URL+"/v1/rooms/"+f.slug+"/viewer", "", f.owner)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "%s", raw)
	require.Contains(t, string(raw), `"notifications":{"available":true,"paused":false,"subscribed":true}`)

	resp = doRoomRequest(t, "GET", f.ts.URL+"/v1/rooms/"+f.slug+"/viewer", "", "")
	raw, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Contains(t, string(raw), `"notifications":{"available":false,"paused":false,"subscribed":false}`,
		"an anonymous reader has nothing to opt in with")
}

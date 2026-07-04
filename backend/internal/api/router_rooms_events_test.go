package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// getJSONWithToken performs a GET with the room bearer token and returns status + body.
func getJSONWithToken(t *testing.T, url, roomToken string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+roomToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return resp.StatusCode, out
}

func eventCount(body map[string]any) int {
	data, _ := body["data"].([]any)
	return len(data)
}

func TestRoomEvents_PostAndQuery(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)

	_, jwt := createRoomTestUser(t, pool)
	slug, roomToken := createTestRoomWithToken(t, ts, jwt)
	eventsURL := ts.URL + "/r/" + slug + "/events"

	post := func(body string) {
		req, _ := http.NewRequest("POST", eventsURL, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+roomToken)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode, "post event: %s", string(raw))
	}
	post(`{"type":"CLAIM","issue":"APP-185","actor":"w1","payload":{"pr":null}}`)
	post(`{"type":"BUILDING","issue":"APP-185","actor":"w1"}`)
	post(`{"type":"CLAIM","issue":"APP-999","actor":"w2"}`)

	// No filter -> all 3.
	status, body := getJSONWithToken(t, eventsURL, roomToken)
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, 3, eventCount(body))

	// Filter by type.
	_, body = getJSONWithToken(t, eventsURL+"?type=CLAIM", roomToken)
	require.Equal(t, 2, eventCount(body))

	// Filter by issue.
	_, body = getJSONWithToken(t, eventsURL+"?issue=APP-185", roomToken)
	require.Equal(t, 2, eventCount(body))

	// Filter by both -> exactly one.
	_, body = getJSONWithToken(t, eventsURL+"?type=CLAIM&issue=APP-185", roomToken)
	require.Equal(t, 1, eventCount(body))

	// Validation: missing type/actor -> 400.
	req, _ := http.NewRequest("POST", eventsURL, strings.NewReader(`{"issue":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+roomToken)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

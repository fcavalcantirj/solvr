package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 step 1: every published example is what the running API answers. The example
// requests are sent in the order an agent would (create a room, join it, post, read, watch,
// then reply to a post) with this run's slug, post id and credentials in place of the
// example's; the answer must have the documented status, validate against the documented
// schema, and show no field the example omits nor a field of another JSON type.
func TestOpenAPIExamples_EachExampleIsWhatTheRunningAPIAnswers(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)

	_, agentKey := registerTestAgent(t, ts, fmt.Sprintf("roomtest_contract_%d", time.Now().UnixNano()%1000000000))
	slug := fmt.Sprintf("test-contract-%d", time.Now().UnixNano()%1000000000)
	status, post := doJSON(t, http.MethodPost, ts.URL+"/v1/posts", agentKey,
		`{"title":"How does a planner hand a plan to an executor agent?","description":"The executor must pick the plan up without a human relaying messages between them."}`)
	require.Equal(t, http.StatusCreated, status, "%v", post)
	postID := post["data"].(map[string]any)["id"].(string)

	live := map[string]string{"slug": slug, "id": postID}
	var roomToken string
	for _, id := range sharedClientOperations {
		ex, ok := examples[id]
		require.True(t, ok, "%s publishes no example", id)

		path := ex.Path
		for name := range ex.PathParams {
			require.NotEmpty(t, live[name], "%s: no live value for path parameter %s", id, name)
			path = strings.ReplaceAll(path, "{"+name+"}", live[name])
		}
		query := url.Values{}
		for name, value := range ex.Query {
			query.Set(name, fmt.Sprint(value))
		}
		target := ts.URL + "/v1" + path
		if len(query) > 0 {
			target += "?" + query.Encode()
		}
		var body []byte
		if ex.Request != nil {
			request := ex.Request
			if m, ok := request.(map[string]interface{}); ok && m["slug"] != nil {
				copied := map[string]interface{}{}
				for k, v := range m {
					copied[k] = v
				}
				copied["slug"] = slug // the example's slug is taken by an earlier run
				request = copied
			}
			var err error
			body, err = json.Marshal(request)
			require.NoError(t, err)
		}
		bearer := map[string]string{"agent_api_key": agentKey, "room_token": roomToken, "none": ""}[ex.Credential]
		require.True(t, ex.Credential == "none" || bearer != "", "%s: no %s yet", id, ex.Credential)

		gotStatus, answer := sendExample(t, ex.Method, target, bearer, body)
		require.Equal(t, ex.Status, fmt.Sprint(gotStatus), "%s %s answered %v", ex.Method, target, answer)
		assert.Empty(t, schemaProblems(spec, answer, ex.ResponseSchema, id+" answer"), "%s: the API breaks its own schema", id)
		assert.Empty(t, shapeProblems(ex.Response, answer, id), "%s: the example does not show what the API answers", id)

		if id == "handshakeRoom" {
			roomToken, _ = answer.(map[string]interface{})["data"].(map[string]interface{})["room_token"].(string)
			require.NotEmpty(t, roomToken)
		}
	}
}

func sendExample(t *testing.T, method, target, bearer string, body []byte) (int, interface{}) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, target, rdr)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer interface{}
	require.NoError(t, json.Unmarshal(raw, &answer), "%s %s: %s", method, target, raw)
	return resp.StatusCode, answer
}

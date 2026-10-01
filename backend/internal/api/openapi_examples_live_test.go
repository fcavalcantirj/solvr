package api

import (
	"bytes"
	"context"
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
// requests are sent in the order an agent would (create a post, read it, create a room, join
// it, post, read, watch, reply to the post, read and edit the reply, then search) with this
// run's slug, ids, credentials and ETag in place of the example's; the answer must have the
// documented status, validate against the documented schema, and show no field the example
// omits nor a field of another JSON type.
func TestOpenAPIExamples_EachExampleIsWhatTheRunningAPIAnswers(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)

	_, agentKey := registerTestAgent(t, ts, fmt.Sprintf("roomtest_contract_%d", time.Now().UnixNano()%1000000000))
	slug := fmt.Sprintf("test-contract-%d", time.Now().UnixNano()%1000000000)
	var postID, replyID, roomToken, etag string
	for _, id := range sharedClientOperations {
		ex, ok := examples[id]
		require.True(t, ok, "%s publishes no example", id)

		path := ex.Path
		for name := range ex.PathParams {
			value := map[string]string{"slug": slug, "id": postID}[name]
			if strings.HasPrefix(ex.Path, "/replies/") {
				value = replyID
			}
			require.NotEmpty(t, value, "%s: no live value for path parameter %s", id, name)
			path = strings.ReplaceAll(path, "{"+name+"}", value)
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
		headers := map[string]string{}
		for name := range ex.Headers {
			require.Equal(t, "If-Match", name, "%s: no live value for header %s", id, name)
			require.NotEmpty(t, etag, "%s: no ETag read before the edit", id)
			headers[name] = etag
		}
		bearer := map[string]string{"agent_api_key": agentKey, "room_token": roomToken, "none": ""}[ex.Credential]
		require.True(t, ex.Credential == "none" || bearer != "", "%s: no %s yet", id, ex.Credential)

		gotStatus, answer, header := sendExample(t, ex.Method, target, bearer, body, headers)
		require.Equal(t, ex.Status, fmt.Sprint(gotStatus), "%s %s answered %v", ex.Method, target, answer)
		assert.Empty(t, schemaProblems(spec, answer, ex.ResponseSchema, id+" answer"), "%s: the API breaks its own schema", id)
		assert.Empty(t, shapeProblems(ex.Response, answer, id), "%s: the example does not show what the API answers", id)

		data, _ := answer.(map[string]interface{})["data"].(map[string]interface{})
		switch id {
		case "createPost":
			postID, _ = data["id"].(string)
			require.NotEmpty(t, postID)
			// What moderation does to a public post before anyone can read or find it.
			_, err := pool.Exec(context.Background(),
				`UPDATE posts SET status = 'open', publication_state = 'published', moderation_state = 'approved' WHERE id = $1`, postID)
			require.NoError(t, err)
		case "handshakeRoom":
			roomToken, _ = data["room_token"].(string)
			require.NotEmpty(t, roomToken)
		case "createReply":
			replyID, _ = data["id"].(string)
			require.NotEmpty(t, replyID)
		case "getReply":
			etag = header.Get("ETag")
		case "search":
			found := false
			for _, item := range answer.(map[string]interface{})["data"].([]interface{}) {
				result := item.(map[string]interface{})
				matches, _ := result["matched_replies"].([]interface{})
				found = found || (result["id"] == postID && len(matches) > 0)
			}
			assert.True(t, found, "search did not find this run's post with its edited reply: %v", answer)
		}
	}
}

func sendExample(t *testing.T, method, target, bearer string, body []byte, headers map[string]string) (int, interface{}, http.Header) {
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
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var answer interface{}
	require.NoError(t, json.Unmarshal(raw, &answer), "%s %s: %s", method, target, raw)
	return resp.StatusCode, answer, resp.Header
}

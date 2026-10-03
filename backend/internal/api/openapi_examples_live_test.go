package api

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78 step 1: every published example is what the running API answers. The example
// requests are sent in the order an agent would (create a post, read it, create a room, join
// it, admit a second agent and list the participants, post, read, watch, reply to the post,
// read and edit the reply, then search) with this run's slug, ids, agents, credentials and
// ETag in place of the example's; the answer must have the
// documented status, validate against the documented schema, and show no field the example
// omits nor a field of another JSON type. The watch reconnects after the entry it posted
// once the room's next entry is stored, and the frame replayed is held to the example frame.
// Then each error example is sent and must answer its status and error code.
func TestOpenAPIExamples_EachExampleIsWhatTheRunningAPIAnswers(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	roomPreCleanup(t, pool)
	spec := servedSpec(t)
	examples := publishedExamples(t, spec)

	ownerID, agentKey := registerTestAgent(t, ts, fmt.Sprintf("roomtest_contract_%d", time.Now().UnixNano()%1000000000))
	// The agent addRoomMember admits in place of the example's.
	admittedID, _ := registerTestAgent(t, ts, fmt.Sprintf("roomtest_cadmit_%d", time.Now().UnixNano()%1000000000))
	slug := fmt.Sprintf("test-contract-%d", time.Now().UnixNano()%1000000000)
	var postID, replyID, roomToken, etag, entryID string
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
			if m, ok := request.(map[string]interface{}); ok && (m["slug"] != nil || m["agent_id"] != nil) {
				copied := map[string]interface{}{}
				for k, v := range m {
					copied[k] = v
				}
				if m["slug"] != nil {
					copied["slug"] = slug // the example's slug is taken by an earlier run
				}
				if m["agent_id"] != nil {
					copied["agent_id"] = admittedID // the example's agent is not registered here
				}
				request = copied
			}
			var err error
			body, err = json.Marshal(request)
			require.NoError(t, err)
		}
		headers := map[string]string{}
		for name := range ex.Headers {
			value := map[string]string{"If-Match": etag, "Last-Event-ID": entryID}[name]
			require.NotEmpty(t, value, "%s: no live value for header %s", id, name)
			headers[name] = value
		}
		bearer := map[string]string{"agent_api_key": agentKey, "room_token": roomToken, "none": ""}[ex.Credential]
		require.True(t, ex.Credential == "none" || bearer != "", "%s: no %s yet", id, ex.Credential)

		if ex.MediaType == "text/event-stream" {
			checkStreamExample(t, ts, spec, ex, slug, roomToken, target, headers)
			continue
		}

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
		case "addRoomMember":
			assert.Equal(t, []interface{}{admittedID, "member", ownerID}, []interface{}{data["agent_id"], data["role"], data["added_by"]},
				"the owner admitted the second agent as a member")
		case "listRoomMembers":
			var got [][]interface{}
			for _, item := range answer.(map[string]interface{})["data"].([]interface{}) {
				m := item.(map[string]interface{})
				got = append(got, []interface{}{m["agent_id"], m["role"], m["added_by"]})
			}
			assert.Equal(t, [][]interface{}{{ownerID, "owner", "system"}, {admittedID, "member", ownerID}}, got,
				"the participants, oldest first, as the example shows them")
		case "createRoomEntry":
			entryID = fmt.Sprint(data["id"])
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

	live := map[string]string{exampleRoomSlug: slug, examplePostID: postID, exampleReplyID: replyID}
	sent := 0
	for _, id := range sharedClientOperations {
		ex := examples[id]
		for _, e := range ex.Errors {
			sent++
			where := id + " error " + e.Status + " (" + e.Case + ")"
			path := ex.Path
			for name, value := range e.PathParams {
				path = strings.ReplaceAll(path, "{"+name+"}", liveValue(live, value))
			}
			query := url.Values{}
			for name, value := range e.Query {
				query.Set(name, liveValue(live, value))
			}
			target := ts.URL + "/v1" + path
			if len(query) > 0 {
				target += "?" + query.Encode()
			}
			var body []byte
			if e.Request != nil {
				var err error
				body, err = json.Marshal(e.Request)
				require.NoError(t, err)
			}
			headers := map[string]string{}
			for name, value := range e.Headers {
				headers[name] = liveValue(live, value)
			}
			bearer := map[string]string{"agent_api_key": agentKey, "room_token": roomToken, "none": "",
				"invalid": "solvr_rt_not-a-live-token"}[e.Credential]
			gotStatus, answer, _ := sendExample(t, ex.Method, target, bearer, body, headers)
			require.Equal(t, e.Status, fmt.Sprint(gotStatus), "%s: %s %s answered %v", where, ex.Method, target, answer)
			assert.Equal(t, errorCode(e.Response), errorCode(answer), "%s: the error code", where)
			assert.Empty(t, schemaProblems(spec, answer, e.ResponseSchema, where+" answer"), where)
			assert.Empty(t, shapeProblems(e.Response, answer, where), where)
		}
	}
	assert.GreaterOrEqual(t, sent, len(errorExampleFamilies))
}

// liveValue is this run's value for a value the examples share (the demo room, post and
// reply); any other value, such as a post id that does not exist or a stale If-Match, is
// sent as written.
func liveValue(live map[string]string, value interface{}) string {
	if v, ok := live[fmt.Sprint(value)]; ok {
		return v
	}
	return fmt.Sprint(value)
}

// checkStreamExample stores the room's next entry, reconnects to the stream after the entry
// the example sequence posted, and holds the first replayed frame to the example frame.
func checkStreamExample(t *testing.T, ts *httptest.Server, spec map[string]interface{}, ex publishedExample,
	slug, roomToken, target string, headers map[string]string) {
	t.Helper()
	want := parseSSE(ex.Response.(string))
	require.Len(t, want, 1, "the stream example shows one frame")
	var wantData map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(want[0].Data), &wantData))
	next, err := json.Marshal(map[string]interface{}{
		"body": wantData["payload"].(map[string]interface{})["content"], "client_entry_id": "build-1",
	})
	require.NoError(t, err)
	status, answer, _ := sendExample(t, "POST", ts.URL+"/v1/rooms/"+slug+"/entries", roomToken, next, nil)
	require.Equal(t, 201, status, "the next entry: %v", answer)
	nextID := answer.(map[string]interface{})["data"].(map[string]interface{})["id"].(float64)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+roomToken)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, ex.Status, fmt.Sprint(resp.StatusCode))
	assert.Equal(t, ex.MediaType, resp.Header.Get("Content-Type"))

	var text strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var got []sseFrame
	for len(got) == 0 && scanner.Scan() {
		text.WriteString(scanner.Text() + "\n")
		if scanner.Text() != "" {
			continue // a frame is complete at its blank line
		}
		for _, f := range parseSSE(text.String()) {
			if f.ID > 0 {
				got = append(got, f)
			}
		}
	}
	require.NoError(t, ctx.Err(), "no frame within 5s: %q", text.String())
	require.NotEmpty(t, got, "the stream ended without a frame: %q", text.String())

	frame := got[0]
	assert.Equal(t, want[0].Event, frame.Event)
	assert.Equal(t, int64(nextID), frame.ID, "the reconnect replays the next entry")
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(frame.Data), &data))
	assert.Empty(t, schemaProblems(spec, data, ref("schemas", "RoomStreamFrame"), "streamRoom frame"))
	assert.Empty(t, schemaProblems(spec, data["payload"], ref("schemas", "RoomStreamMessage"), "streamRoom frame.payload"))
	assert.Empty(t, shapeProblems(wantData, data, "streamRoom frame"), "the example frame does not show what the stream sends")
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

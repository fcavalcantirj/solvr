package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// idx 78: POST /v1/mcp serves a tool for each contract operation and runs it through the real
// router and database. Three independently registered agents (a planner, an executor and a
// reviewer) work one room and one post only through /v1/mcp, each with its own API key and its
// own room token; nothing is relayed by a human.

// mcpHTTP calls a tool through POST /v1/mcp with key as the bearer ("" = anonymous).
func mcpHTTP(t *testing.T, base, key, tool string, args map[string]any) (string, bool) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/mcp", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err, tool)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, tool)
	var out struct {
		Result struct {
			Content []struct{ Text string } `json:"content"`
			IsError bool                    `json:"isError"`
		} `json:"result"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out), tool)
	require.Len(t, out.Result.Content, 1, tool)
	return out.Result.Content[0].Text, out.Result.IsError
}

func mcpOK(t *testing.T, base, key, tool string, args map[string]any) string {
	t.Helper()
	text, isError := mcpHTTP(t, base, key, tool, args)
	require.False(t, isError, "%s: %s", tool, text)
	return text
}

func mcpCapture(t *testing.T, text, pattern string) string {
	t.Helper()
	m := regexp.MustCompile(pattern).FindStringSubmatch(text)
	require.NotNil(t, m, "%q in:\n%s", pattern, text)
	return m[1]
}

func TestMCP_ThreeAgentsWorkARoomAndAPostThroughTheRealRouter(t *testing.T) {
	ts, _, cleanup := setupRoomTestServer(t)
	defer cleanup()
	base := ts.URL

	type agent struct{ id, key, token string }
	agents := map[string]*agent{}
	for _, role := range []string{"planner", "executor", "reviewer"} {
		id, key := registerRoomTestAgent(t, ts)
		agents[role] = &agent{id: id, key: key}
	}
	planner, executor, reviewer := agents["planner"], agents["executor"], agents["reviewer"]

	slug := fmt.Sprintf("test-mcp-%d", time.Now().UnixNano())
	created := mcpOK(t, base, planner.key, "solvr_room_create", map[string]any{"display_name": "MCP plan build review", "slug": slug})
	assert.Contains(t, created, "Room "+slug+" created")

	for role, a := range agents {
		joined := mcpOK(t, base, a.key, "solvr_room_join", map[string]any{"slug": slug})
		a.token = mcpCapture(t, joined, `Room token: (\S+)`)
		assert.Contains(t, joined, "as "+a.id, role)
	}

	hello := mcpOK(t, base, "", "solvr_room_send", map[string]any{"slug": slug, "body": "hello", "room_token": planner.token})
	helloID := mcpCapture(t, hello, `Message (\d+) sent`)

	// Both watchers wait while the planner sends the plan; each wakes with it (replayed after
	// hello if it was sent before the watch subscribed, live otherwise).
	var wg sync.WaitGroup
	watched := map[string]string{}
	var mu sync.Mutex
	for _, role := range []string{"executor", "reviewer"} {
		wg.Add(1)
		go func(role string) {
			defer wg.Done()
			text, isError := mcpHTTP(t, base, agents[role].key, "solvr_room_watch", map[string]any{
				"slug": slug, "room_token": agents[role].token, "last_event_id": helloID, "event_type": "message", "wait_seconds": 10})
			mu.Lock()
			watched[role] = fmt.Sprintf("%v %s", isError, text)
			mu.Unlock()
		}(role)
	}
	time.Sleep(300 * time.Millisecond)
	plan := mcpOK(t, base, "", "solvr_room_send", map[string]any{"slug": slug, "body": "Plan: build the parser, then review it.",
		"room_token": planner.token, "client_entry_id": "plan-1", "addressed_member_ids": []string{executor.id, reviewer.id}})
	planID := mcpCapture(t, plan, `Message (\d+) sent`)
	wg.Wait()
	for _, role := range []string{"executor", "reviewer"} {
		assert.Contains(t, watched[role], "false #", role)
		assert.Contains(t, watched[role], "(id "+planID+") "+planner.id+": Plan: build the parser", role)
		assert.Contains(t, watched[role], "last_event_id "+planID, role)
	}

	replay := mcpOK(t, base, "", "solvr_room_send", map[string]any{"slug": slug, "body": "Plan: build the parser, then review it.",
		"room_token": planner.token, "client_entry_id": "plan-1", "addressed_member_ids": []string{executor.id, reviewer.id}})
	assert.Contains(t, replay, "Message "+planID+" was already sent")

	done := mcpOK(t, base, "", "solvr_room_send", map[string]any{"slug": slug, "body": "Built; tests pass.",
		"room_token": executor.token, "reply_to_entry_id": planID})
	doneID := mcpCapture(t, done, `Message (\d+) sent`)

	read := mcpOK(t, base, reviewer.key, "solvr_room_read", map[string]any{"slug": slug, "room_token": reviewer.token, "kind": "message"})
	assert.Contains(t, read, "to "+executor.id+", "+reviewer.id)
	assert.Contains(t, read, "(id "+doneID+") "+executor.id+" (reply to "+planID+"): Built; tests pass.")

	ticket := mcpCapture(t, mcpOK(t, base, reviewer.key, "solvr_room_ticket", map[string]any{"slug": slug, "room_token": reviewer.token}), `Ticket: (\S+)`)
	byTicket := mcpOK(t, base, "", "solvr_room_watch", map[string]any{"slug": slug, "ticket": ticket, "last_event_id": planID, "event_type": "message", "wait_seconds": 5})
	assert.Contains(t, byTicket, "(id "+doneID+") "+executor.id+": Built; tests pass.")

	text, isError := mcpHTTP(t, base, reviewer.key, "solvr_room_read", map[string]any{"slug": slug, "room_token": "solvr_rt_not_a_live_token"})
	assert.True(t, isError)
	assert.Contains(t, text, "API request failed: 401 Unauthorized: UNAUTHORIZED")
	assert.Regexp(t, `request id: \S+`, text)

	quiet := mcpOK(t, base, "", "solvr_room_watch", map[string]any{"slug": slug, "room_token": reviewer.token, "last_event_id": doneID, "event_type": "message", "wait_seconds": 1})
	assert.Contains(t, quiet, "No events within 1 s.")

	// Knowledge: the planner posts, the executor replies and edits with the ETag it read.
	post := mcpOK(t, base, planner.key, "solvr_post", map[string]any{"title": "MCP planner handoff " + slug,
		"description": "How a planner hands a plan to an executor through one room.", "tags": []string{"mcp"}})
	postID := mcpCapture(t, post, `ID: (\S+)`)
	reply := mcpOK(t, base, executor.key, "solvr_reply", map[string]any{"post_id": postID, "body": "Join the room and watch it."})
	replyID := mcpCapture(t, reply, `ID: (\S+)`)

	got := mcpOK(t, base, executor.key, "solvr_get_reply", map[string]any{"id": replyID})
	etag := mcpCapture(t, got, `ETag: (\S+)`)
	updated := mcpOK(t, base, executor.key, "solvr_update_reply", map[string]any{"id": replyID, "if_match": etag, "body": "Join the room, then watch it."})
	assert.Contains(t, updated, "Reply "+replyID+" updated.")
	text, isError = mcpHTTP(t, base, executor.key, "solvr_update_reply", map[string]any{"id": replyID, "if_match": etag, "body": "stale"})
	assert.True(t, isError)
	assert.Contains(t, text, "PRECONDITION_FAILED")

	full := mcpOK(t, base, planner.key, "solvr_get", map[string]any{"id": postID})
	assert.Contains(t, full, "## Replies (1)")
	assert.Contains(t, full, "Join the room, then watch it.")
	listed := mcpOK(t, base, planner.key, "solvr_replies", map[string]any{"post_id": postID, "limit": 5})
	assert.Contains(t, listed, "["+replyID+"] agent "+executor.id)

	text, isError = mcpHTTP(t, base, "", "solvr_search", map[string]any{"query": ""})
	assert.True(t, isError)
	assert.Contains(t, text, "VALIDATION_ERROR")
	searched := mcpOK(t, base, "", "solvr_search", map[string]any{"query": "planner handoff", "sort": "newest", "limit": 3})
	assert.True(t, strings.HasPrefix(searched, "Found ") || strings.HasPrefix(searched, "No results found"), searched)
}

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

const mcpTestKey = "Bearer solvr_test_agent_key"

// mcpSearchThroughTheAPI serves GET /v1/search with the real search handler over a mock
// repository, so the tool text is built from the API's own answer.
func mcpSearchThroughTheAPI(repo *MockSearchRepository) (*MCPHandler, *mcpRecorder) {
	router := chi.NewRouter()
	router.Get("/v1/search", NewSearchHandler(repo).Search)
	return recordMCP(router.ServeHTTP)
}

// Replaces TestMCPExecuteSearch_NoConfidentMatch (the repository-backed executeSearch): the
// search handler's confident_match drives the ASK guidance in the tool text.
func TestMCPSearch_NoConfidentMatchShowsTheAskGuidance(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "post", Title: "weak match", Similarity: ptrFloat64(0.4)}}, 1)
	repo.SetTopSimilarity(ptrFloat64(0.4))
	h, _ := mcpSearchThroughTheAPI(repo) // default threshold 0.85: 0.4 is not confident

	text, isError := callMCP(t, h, "solvr_search", map[string]interface{}{"query": "race condition"}, "")
	require.False(t, isError, text)
	assert.Contains(t, text, "No confident match")
	assert.Contains(t, text, "Similarity: 40% (semantic)")
}

// Replaces TestMCPExecuteSearch_ConfidentMatch.
func TestMCPSearch_ConfidentMatchOmitsTheAskGuidance(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{{ID: "p1", Type: "post", Title: "strong match", Similarity: ptrFloat64(0.92)}}, 1)
	repo.SetTopSimilarity(ptrFloat64(0.92))
	h, _ := mcpSearchThroughTheAPI(repo)

	text, isError := callMCP(t, h, "solvr_search", map[string]interface{}{"query": "race condition"}, "")
	require.False(t, isError, text)
	assert.NotContains(t, text, "No confident match")
	assert.Contains(t, text, "strong match")
}

// Replaces TestMCPSearch_IgnoresALegacyTypeArgument (idx 78, 2.0.0): a legacy type from a 1.x
// client is refused before any request instead of being dropped; a 2.0.0 search sends no type,
// so it never narrows the search, and the reply anchors still show.
func TestMCPSearch_RefusesALegacyTypeArgumentBeforeAnyRequest(t *testing.T) {
	repo := NewMockSearchRepository()
	repo.SetResults([]models.SearchResult{canonicalSearchResult()}, 1)
	h, rec := mcpSearchThroughTheAPI(repo)

	text, isError := callMCP(t, h, "solvr_search", map[string]interface{}{"query": "failed", "type": "problem"}, "")
	require.True(t, isError, text)
	assert.Contains(t, text, "The 'type' argument of solvr_search was removed in /v1/mcp 2.0.0; search covers every post")
	assert.Empty(t, rec.requests(), "a refused search sends no request")

	text, isError = callMCP(t, h, "solvr_search", map[string]interface{}{"query": "failed"}, "")
	require.False(t, isError, text)
	require.Len(t, rec.requests(), 1)
	assert.NotContains(t, rec.requests()[0].query, "type")
	assert.Empty(t, repo.searchOpts.Type, "the legacy type argument is not a filter")
	assert.Empty(t, repo.searchOpts.ContentTypes)
	assert.Contains(t, text, "Matched reply: /posts/post-1#reply-1 by Agent Two (approach, failed)")
}

func TestMCPTools_DispatchWithTheCallersRequestIDAndAddress(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}, "meta": map[string]interface{}{"total": 0}})
	})
	text, _ := callMCP(t, h, "solvr_search", map[string]interface{}{"query": "x"}, "")
	require.Len(t, rec.requests(), 1, text)
	sent := rec.requests()[0]
	assert.Equal(t, "mcp-call-1", sent.header.Get("X-Request-ID"))
	assert.Equal(t, "203.0.113.9:4567", sent.remoteAddr)
	assert.True(t, sent.inProcess)
	assert.Contains(t, text, "No results found")
}

func TestMCPTools_WithoutTheAPIFail(t *testing.T) {
	text, isError := callMCP(t, NewMCPHandler(nil), "solvr_search", map[string]interface{}{"query": "x"}, "")
	assert.True(t, isError)
	assert.Contains(t, text, "not connected to the API")
}

// With the caller's API key, solvr_post and solvr_reply create through the API (the key is the
// bearer); the post body carries no type. (2.0.0 refuses a legacy type before any request:
// TestMCPRemoved_Refuses1xCallsBeforeAnyRequest.)
func TestMCPPostAndReply_WithAKeyCreateThroughTheAPI(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/replies") {
			writeMCPJSON(w, http.StatusCreated, map[string]interface{}{"data": map[string]interface{}{"id": "rep-9", "parent_reply_id": "rep-1"}})
			return
		}
		writeMCPJSON(w, http.StatusCreated, map[string]interface{}{"data": map[string]interface{}{"id": "post-9", "title": "Pool", "status": "pending_review"}})
	})

	text, isError := callMCP(t, h, "solvr_post", map[string]interface{}{"title": "Pool", "description": "Exhausted."}, mcpTestKey)
	require.False(t, isError, text)
	assert.Contains(t, text, "Created post: Pool")
	assert.Contains(t, text, "ID: post-9")
	text, isError = callMCP(t, h, "solvr_reply", map[string]interface{}{"post_id": "post-9", "body": "Raise it.", "parent_reply_id": "rep-1"}, mcpTestKey)
	require.False(t, isError, text)
	assert.Contains(t, text, "ID: rep-9")
	assert.Contains(t, text, "in reply to rep-1")

	sent := rec.requests()
	require.Len(t, sent, 2)
	assert.Equal(t, "/v1/posts", sent[0].path)
	assert.JSONEq(t, `{"title":"Pool","description":"Exhausted."}`, sent[0].body)
	assert.Equal(t, "/v1/posts/post-9/replies", sent[1].path)
	assert.JSONEq(t, `{"body":"Raise it.","parent_reply_id":"rep-1"}`, sent[1].body)
	for _, s := range sent {
		assert.Equal(t, mcpTestKey, s.auth)
	}
}

func TestMCPRoomTools_WithoutARoomTokenFailBeforeAnyRequest(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	for _, tool := range []string{"solvr_room_read", "solvr_room_send", "solvr_room_ticket", "solvr_room_watch"} {
		text, isError := callMCP(t, h, tool, map[string]interface{}{"slug": "demo", "body": "hi"}, mcpTestKey)
		assert.True(t, isError, tool)
		assert.Contains(t, text, `No room token for demo. Call solvr_room_join with slug "demo"`, tool)
	}
	for _, missing := range []map[string]interface{}{{"room_token": "solvr_rt_x"}, {"slug": "demo", "room_token": "solvr_rt_x"}} {
		text, isError := callMCP(t, h, "solvr_room_send", missing, mcpTestKey)
		assert.True(t, isError)
		assert.Contains(t, text, "is required")
	}
	assert.Empty(t, rec.requests(), "nothing is sent without a room token or a required argument")
}

func TestMCPTools_EscapePathParameters(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": []interface{}{}, "meta": map[string]interface{}{}})
	})
	callMCP(t, h, "solvr_room_read", map[string]interface{}{"slug": "a b/c", "room_token": "solvr_rt_x"}, "")
	callMCP(t, h, "solvr_get_reply", map[string]interface{}{"id": "rep/1"}, "")
	sent := rec.requests()
	require.Len(t, sent, 2)
	assert.Equal(t, "/v1/rooms/a%20b%2Fc/entries", sent[0].path)
	assert.Equal(t, "/v1/replies/rep%2F1", sent[1].path)
}

func TestMCPRoomJoin_SendsOnlyTheGivenFieldsAndShowsTheToken(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusCreated, map[string]interface{}{"data": map[string]interface{}{
			"agent_id": "agent_a", "room_slug": "demo", "room_token": "solvr_rt_new", "rotated": true}})
	})
	text, isError := callMCP(t, h, "solvr_room_join", map[string]interface{}{"slug": "demo", "rotate": true, "ttl_seconds": 60.0}, mcpTestKey)
	require.False(t, isError, text)
	assert.Contains(t, text, "Room token: solvr_rt_new")
	assert.Contains(t, text, "rotated")
	callMCP(t, h, "solvr_room_join", map[string]interface{}{"slug": "demo"}, mcpTestKey)
	sent := rec.requests()
	require.Len(t, sent, 2)
	assert.JSONEq(t, `{"rotate":true,"ttl_seconds":60}`, sent[0].body)
	assert.JSONEq(t, `{}`, sent[1].body)
	assert.Equal(t, mcpTestKey, sent[0].auth)
}

func TestMCPRoomSend_ReportsAReplayedClientEntryID(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"id": 42, "sequence": 3},
			"meta": map[string]interface{}{"idempotent_replay": true}})
	})
	text, isError := callMCP(t, h, "solvr_room_send", map[string]interface{}{"slug": "demo", "body": "plan", "room_token": "solvr_rt_x",
		"client_entry_id": "plan-1", "reply_to_entry_id": 7.0, "addressed_member_ids": []interface{}{"agent_b", "agent_c"}}, mcpTestKey)
	require.False(t, isError, text)
	assert.Contains(t, text, "Message 42 was already sent")
	assert.JSONEq(t, `{"body":"plan","client_entry_id":"plan-1","reply_to_entry_id":7,"addressed_member_ids":["agent_b","agent_c"]}`, rec.requests()[0].body)
}

// sseFrame is one message frame as the room stream writes it.
func sseFrame(id int, agent, content string) string {
	data, _ := json.Marshal(map[string]interface{}{"id": id, "sequence": id, "type": "message", "agent_name": agent,
		"payload": map[string]interface{}{"agent_name": agent, "content": content, "sequence_num": id}})
	return fmt.Sprintf("id: %d\nevent: message\ndata: %s\n\n", id, data)
}

// streamThenHold writes the frames, flushes, and holds the stream open until the watch closes it.
func streamThenHold(frames ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, f := range frames {
			_, _ = w.Write([]byte(f))
			w.(http.Flusher).Flush()
		}
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}
}

func TestMCPRoomWatch_StopsAfterMaxEventsOnAnOpenStream(t *testing.T) {
	h, rec := recordMCP(streamThenHold(": heartbeat\n\n", sseFrame(1, "planner", "plan"), sseFrame(2, "executor", "done"), sseFrame(3, "reviewer", "late")))
	start := time.Now()
	text, isError := callMCP(t, h, "solvr_room_watch", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x", "max_events": 2.0, "event_type": "message"}, mcpTestKey)
	require.False(t, isError, text)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.Contains(t, text, "#1 (id 1) planner: plan")
	assert.Contains(t, text, "#2 (id 2) executor: done")
	assert.NotContains(t, text, "late")
	assert.Contains(t, text, "To continue: solvr_room_watch with slug demo and last_event_id 2")
	sent := rec.requests()[0]
	assert.Equal(t, "text/event-stream", sent.header.Get("Accept"))
	assert.Equal(t, "Bearer solvr_rt_x", sent.auth)
	assert.Equal(t, []string{"message"}, sent.query["type"])
}

func TestMCPRoomWatch_AQuietStreamAnswersAfterWaitSeconds(t *testing.T) {
	h, _ := recordMCP(streamThenHold())
	start := time.Now()
	text, isError := callMCP(t, h, "solvr_room_watch", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x", "wait_seconds": 1.0, "last_event_id": "9"}, mcpTestKey)
	assert.False(t, isError, text)
	assert.Less(t, time.Since(start), 4*time.Second)
	assert.Contains(t, text, "No events within 1 s.")
	assert.Contains(t, text, "last_event_id 9")
}

func TestMCPRoomWatch_AStreamEndIsAnErrorThatKeepsTheEarlierEvents(t *testing.T) {
	end := "event: credential_rotated\ndata: {\"code\":\"CREDENTIAL_ROTATED\",\"message\":\"a handshake replaced this token\"}\n\n"
	h, _ := recordMCP(streamThenHold(sseFrame(5, "planner", "plan"), end))
	text, isError := callMCP(t, h, "solvr_room_watch", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x", "max_events": 3.0}, mcpTestKey)
	assert.True(t, isError)
	assert.Contains(t, text, "#5 (id 5) planner: plan")
	assert.Contains(t, text, "Stream ended: CREDENTIAL_ROTATED: a handshake replaced this token")

	h, _ = recordMCP(streamThenHold(end))
	text, isError = callMCP(t, h, "solvr_room_watch", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x"}, mcpTestKey)
	assert.True(t, isError)
	assert.Contains(t, text, "Error executing solvr_room_watch: Stream ended: CREDENTIAL_ROTATED")
}

func TestMCPRoomWatch_WithATicketIsAnonymous(t *testing.T) {
	h, rec := recordMCP(streamThenHold(sseFrame(8, "planner", "plan")))
	text, isError := callMCP(t, h, "solvr_room_watch", map[string]interface{}{"slug": "demo", "ticket": "solvr_st_t", "room_token": "solvr_rt_x"}, mcpTestKey)
	require.False(t, isError, text)
	sent := rec.requests()[0]
	assert.Empty(t, sent.auth, "a ticket watch presents no credential")
	assert.Equal(t, []string{"solvr_st_t"}, sent.query["ticket"])
}

func TestMCPRoomWatch_AStreamThatEndsWithNoEvent(t *testing.T) {
	h, _ := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("retry: 1000\n\n"))
	})
	text, isError := callMCP(t, h, "solvr_room_watch", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x"}, mcpTestKey)
	assert.False(t, isError)
	assert.Contains(t, text, "No events: the stream ended.")
}

type mcpOuterKey struct{}

// The router that serves /v1/mcp also serves the dispatched request: it is routed from the
// root (not as a sub-router call of the MCP request) and sees no value of the MCP request.
func TestMCPTools_DispatchThroughTheRouterThatServesThem(t *testing.T) {
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/v1/mcp" {
				r = r.WithContext(context.WithValue(r.Context(), mcpOuterKey{}, "caller identity"))
			}
			next.ServeHTTP(w, r)
		})
	})
	h := NewMCPHandler(router)
	router.Post("/v1/mcp", h.Handle)
	var leaked interface{}
	router.Get("/v1/replies/{id}", func(w http.ResponseWriter, r *http.Request) {
		leaked = r.Context().Value(mcpOuterKey{})
		writeMCPJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"id": chi.URLParam(r, "id"), "body": "routed"}})
	})

	raw, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]interface{}{"name": "solvr_get_reply", "arguments": map[string]interface{}{"id": "rep-7"}}})
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/v1/mcp", bytes.NewReader(raw)))
	assert.Contains(t, rr.Body.String(), "Reply rep-7")
	assert.Contains(t, rr.Body.String(), "routed")
	assert.Nil(t, leaked, "a value of the MCP request reached the dispatched request")
}

func TestMCPRoomRead_SendsItsFiltersAndShowsTheNextCursor(t *testing.T) {
	h, rec := recordMCP(func(w http.ResponseWriter, _ *http.Request) {
		writeMCPJSON(w, http.StatusOK, map[string]interface{}{
			"data": []interface{}{map[string]interface{}{"id": 9, "sequence": 4, "kind": "event", "actor_label": "agent_a",
				"event_type": "CLAIM", "issue": "parser", "addressed_member_ids": []string{"agent_b"}}},
			"meta": map[string]interface{}{"has_more": true, "next_cursor": "c2"}})
	})
	text, isError := callMCP(t, h, "solvr_room_read", map[string]interface{}{"slug": "demo", "room_token": "solvr_rt_x",
		"kind": "event", "issue": "parser", "cursor": "c1", "limit": 10.0}, "")
	require.False(t, isError, text)
	assert.Equal(t, map[string][]string{"kind": {"event"}, "issue": {"parser"}, "cursor": {"c1"}, "limit": {"10"}}, rec.requests()[0].query)
	assert.Contains(t, text, "#4 (id 9) agent_a [CLAIM] issue parser (to agent_b)")
	assert.Contains(t, text, "More: call solvr_room_read with slug demo and cursor c2")
}

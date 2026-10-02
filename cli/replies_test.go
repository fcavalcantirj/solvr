package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// repliesPage builds a GET /posts/{id}/replies response body.
func repliesPage(replies []map[string]interface{}, meta map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"data": replies, "meta": meta}
}

// TestGetCommand_NoIncludeFlag verifies get no longer fetches typed
// contributions: --include is removed (hidden; 0.2.0 refuses it naming
// solvr replies, migration_test.go) and an old invocation sends nothing.
func TestGetCommand_NoIncludeFlag(t *testing.T) {
	getCmd := NewGetCmd()
	if flag := getCmd.Flags().Lookup("include"); flag == nil || !flag.Hidden {
		t.Error("get command should offer no --include flag (replies are read with solvr replies)")
	}

	isolateHome(t)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "q-123", "--include", "answers", "--api-url", server.URL})

	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "'--include' was removed") || !strings.Contains(err.Error(), "solvr replies <id>") {
		t.Errorf("expected the removed-flag error naming solvr replies, got: %v", err)
	}
	if called {
		t.Error("get --include must not call the API")
	}
}

// TestRepliesCommand_ListsReplies verifies the replies of a post are read from
// the canonical replies endpoint, migrated contributions included.
func TestRepliesCommand_ListsReplies(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if r.Method != "GET" || r.URL.Path != "/posts/prob-123/replies" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(repliesPage([]map[string]interface{}{
			{
				"id": "reply-1", "post_id": "prob-123", "author_type": "human", "author_id": "user-2",
				"body":  "Use mutex locks: wrap database access in sync.Mutex",
				"score": 3, "legacy_type": "approach", "legacy_id": "approach-1",
			},
			{
				"id": "reply-2", "post_id": "prob-123", "author_type": "agent", "author_id": "agent_claude",
				"body":  "A channel per worker avoids the lock entirely.",
				"score": 1,
			},
		}, map[string]interface{}{"total": 2, "has_more": false}))
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"replies", "prob-123", "--api-url", server.URL, "--api-key", "test_key"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("replies command failed: %v", err)
	}

	if len(paths) != 1 || paths[0] != "/posts/prob-123/replies" {
		t.Errorf("expected exactly one call to /posts/prob-123/replies, got %v", paths)
	}

	output := buf.String()
	for _, want := range []string{
		"2 replies",
		"reply-1",
		"user-2 (human)",
		"Score: 3",
		"migrated approach",
		"Use mutex locks: wrap database access in sync.Mutex",
		"reply-2",
		"agent_claude (agent)",
		"A channel per worker avoids the lock entirely.",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %s", want, output)
		}
	}
	if strings.Count(output, "migrated") != 1 {
		t.Errorf("only the migrated reply should be labeled, got: %s", output)
	}
}

// TestRepliesCommand_MarksThreadedReplies verifies a reply threaded under
// another reply names its parent.
func TestRepliesCommand_MarksThreadedReplies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(repliesPage([]map[string]interface{}{
			{"id": "reply-1", "post_id": "q-123", "author_id": "user-2", "author_type": "human",
				"body": "Use channels for synchronization"},
			{"id": "reply-2", "post_id": "q-123", "parent_reply_id": "reply-1", "author_id": "user-3", "author_type": "human",
				"body": "Agreed, and close them from the sender"},
		}, map[string]interface{}{"total": 2, "has_more": false}))
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"replies", "q-123", "--api-url", server.URL, "--api-key", "test_key"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("replies command failed: %v", err)
	}

	output := buf.String()
	if strings.Count(output, "in reply to reply-1") != 1 {
		t.Errorf("expected the threaded reply to name its parent once, got: %s", output)
	}
	if !strings.Contains(output, "Use channels for synchronization") || !strings.Contains(output, "close them from the sender") {
		t.Errorf("expected both reply bodies, got: %s", output)
	}
}

// TestRepliesCommand_NoReplies verifies an empty page says so.
func TestRepliesCommand_NoReplies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(repliesPage([]map[string]interface{}{}, map[string]interface{}{"total": 0, "has_more": false}))
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"replies", "idea-123", "--api-url", server.URL, "--api-key", "test_key"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("replies command failed: %v", err)
	}
	if !strings.Contains(buf.String(), "No replies yet") {
		t.Errorf("expected 'No replies yet', got: %s", buf.String())
	}
}

// TestRepliesCommand_PagesWithLimitAndCursor verifies --limit and --cursor are
// sent as the API's paging parameters and the next page is announced.
func TestRepliesCommand_PagesWithLimitAndCursor(t *testing.T) {
	var gotLimit, gotCursor string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLimit = r.URL.Query().Get("limit")
		gotCursor = r.URL.Query().Get("cursor")
		json.NewEncoder(w).Encode(repliesPage([]map[string]interface{}{
			{"id": "reply-5", "post_id": "p-1", "author_id": "user-1", "author_type": "human", "body": "Fifth"},
		}, map[string]interface{}{"total": 9, "has_more": true, "next_cursor": "cur_next"}))
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"replies", "p-1", "--limit", "1", "--cursor", "cur_prev", "--api-url", server.URL, "--api-key", "test_key"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("replies command failed: %v", err)
	}
	if gotLimit != "1" || gotCursor != "cur_prev" {
		t.Errorf("expected limit=1 cursor=cur_prev, got limit=%q cursor=%q", gotLimit, gotCursor)
	}
	output := buf.String()
	if !strings.Contains(output, "Showing 1 of 9 replies") {
		t.Errorf("expected the page position, got: %s", output)
	}
	if !strings.Contains(output, "More: solvr replies p-1 --cursor cur_next") {
		t.Errorf("expected the next page hint, got: %s", output)
	}
}

// TestRepliesCommand_FirstPageSendsNoCursor verifies no paging parameter is
// sent unless asked for, and no next-page hint is printed on the last page.
func TestRepliesCommand_FirstPageSendsNoCursor(t *testing.T) {
	var rawQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(repliesPage([]map[string]interface{}{
			{"id": "reply-1", "post_id": "p-1", "author_id": "user-1", "author_type": "human", "body": "Only"},
		}, map[string]interface{}{"total": 1, "has_more": false}))
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"replies", "p-1", "--api-url", server.URL, "--api-key", "test_key"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("replies command failed: %v", err)
	}
	if rawQuery != "" {
		t.Errorf("expected no query parameters, got %q", rawQuery)
	}
	if strings.Contains(buf.String(), "More:") {
		t.Errorf("the last page has no next-page hint, got: %s", buf.String())
	}
}

// TestRepliesCommand_HelpText verifies help explains paging and get points to replies
func TestRepliesCommand_HelpText(t *testing.T) {
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"replies", "--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()
	for _, want := range []string{"replies <post_id>", "--limit", "--cursor", "--json"} {
		if !strings.Contains(output, want) {
			t.Errorf("replies help should mention %q, got: %s", want, output)
		}
	}

	getCmd := NewGetCmd()
	if !strings.Contains(getCmd.Long, "solvr replies") {
		t.Errorf("get help should point to 'solvr replies', got: %s", getCmd.Long)
	}
}

// TestRepliesCommand_JSONOutput verifies --json prints the API page as returned
func TestRepliesCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(repliesPage([]map[string]interface{}{
			{"id": "reply-1", "post_id": "q-123", "author_id": "user-2", "author_type": "human",
				"body": "Test reply", "score": 2},
		}, map[string]interface{}{"total": 1, "has_more": false}))
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"replies", "q-123", "--json", "--api-url", server.URL, "--api-key", "test_key"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("replies command failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output should be valid JSON: %v\nOutput was: %s", err, buf.String())
	}
	data, ok := result["data"].([]interface{})
	if !ok || len(data) != 1 {
		t.Fatalf("expected a data array with 1 reply, got: %s", buf.String())
	}
	first := data[0].(map[string]interface{})
	if first["body"] != "Test reply" {
		t.Errorf("expected the reply body, got %v", first["body"])
	}
	meta, ok := result["meta"].(map[string]interface{})
	if !ok || meta["total"] != float64(1) {
		t.Errorf("expected meta.total 1, got: %s", buf.String())
	}
}

// TestRepliesCommand_APIError verifies API errors carry code and message
func TestRepliesCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{"code": "NOT_FOUND", "message": "post not found"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"replies", "missing", "--api-url", server.URL, "--api-key", "test_key"})

	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "NOT_FOUND: post not found") {
		t.Errorf("expected 'NOT_FOUND: post not found', got: %v", err)
	}
}

// TestRepliesCommand_RequiresPostID verifies the post ID argument is required
func TestRepliesCommand_RequiresPostID(t *testing.T) {
	isolateHome(t)
	rootCmd := NewRootCmd()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"replies", "--api-url", unreachableAPI})

	err := rootCmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "accepts 1 arg") {
		t.Errorf("expected an argument count error, got: %v", err)
	}
}

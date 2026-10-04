package main

/**
 * E2E tests for CLI get command
 *
 * Per PRD line 5052-5057:
 * - E2E: CLI commands
 * - Test solvr get
 *
 * These tests verify the full CLI command execution flow for get.
 */

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// E2E Test: solvr get command
// ============================================================================

func TestE2E_GetCommand_Problem(t *testing.T) {
	// Setup mock API server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify request path
		if r.URL.Path != "/v1/posts/prob-123" {
			t.Errorf("expected path /v1/posts/prob-123, got %s", r.URL.Path)
		}

		weight := 4
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "prob-123",
				Type:        "problem",
				Title:       "Memory leak in async handler",
				Description: "The async handler is leaking memory when processing large payloads...",
				Tags:        []string{"go", "async", "memory"},
				Status:      "open",
				Author: AuthorInfo{
					ID:          "agent_claude",
					Type:        "agent",
					DisplayName: "Claude Assistant",
				},
				Upvotes:         10,
				Downvotes:       2,
				VoteScore:       8,
				SuccessCriteria: []string{"No memory growth after 1000 requests", "Response time under 100ms"},
				Weight:          &weight,
				CreatedAt:       time.Now().Add(-72 * time.Hour),
				UpdatedAt:       time.Now().Add(-24 * time.Hour),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Execute CLI command
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "prob-123"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify API was called
	if requestCount != 1 {
		t.Errorf("expected 1 API request, got %d", requestCount)
	}

	output := stdout.String()

	// Verify output contains expected fields
	if !strings.Contains(output, "Memory leak in async handler") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "problem") {
		t.Error("output should contain post type")
	}
	if !strings.Contains(output, "prob-123") {
		t.Error("output should contain post ID")
	}
	if !strings.Contains(output, "Claude Assistant") {
		t.Error("output should contain author name")
	}
	if !strings.Contains(output, "Weight") {
		t.Error("output should contain weight for problem")
	}
	if !strings.Contains(output, "Success Criteria") {
		t.Error("output should contain success criteria for problem")
	}
}

func TestE2E_GetCommand_Question(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/posts/q-456" {
			t.Errorf("expected path /v1/posts/q-456, got %s", r.URL.Path)
		}

		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "q-456",
				Type:        "question",
				Title:       "How to implement retry logic?",
				Description: "I need to implement exponential backoff for API calls...",
				Tags:        []string{"go", "retry", "api"},
				Status:      "answered",
				Author: AuthorInfo{
					ID:          "user-789",
					Type:        "human",
					DisplayName: "John Developer",
				},
				Upvotes:   25,
				Downvotes: 1,
				VoteScore: 24,
				CreatedAt: time.Now().Add(-48 * time.Hour),
				UpdatedAt: time.Now().Add(-12 * time.Hour),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "q-456"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "How to implement retry logic?") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "question") {
		t.Error("output should contain post type")
	}
	if !strings.Contains(output, "answered") {
		t.Error("output should contain status")
	}
}

func TestE2E_GetCommand_Idea(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/posts/idea-789" {
			t.Errorf("expected path /v1/posts/idea-789, got %s", r.URL.Path)
		}

		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "idea-789",
				Type:        "idea",
				Title:       "New caching strategy for search",
				Description: "What if we implement a two-tier caching system...",
				Tags:        []string{"caching", "performance", "search"},
				Status:      "exploring",
				Author: AuthorInfo{
					ID:          "agent_gpt4",
					Type:        "agent",
					DisplayName: "GPT-4 Assistant",
				},
				Upvotes:   15,
				Downvotes: 3,
				VoteScore: 12,
				CreatedAt: time.Now().Add(-24 * time.Hour),
				UpdatedAt: time.Now().Add(-6 * time.Hour),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "idea-789"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "New caching strategy for search") {
		t.Error("output should contain title")
	}
	if !strings.Contains(output, "idea") {
		t.Error("output should contain post type")
	}
}

func TestE2E_GetCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-json",
				Type:        "question",
				Title:       "JSON output test",
				Description: "Testing JSON output format",
				Tags:        []string{"test"},
				Status:      "open",
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "Tester",
				},
				Upvotes:   5,
				Downvotes: 0,
				VoteScore: 5,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "--json", "post-json"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify output is valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data field in JSON response")
	}
	if data["id"] != "post-json" {
		t.Errorf("expected ID 'post-json', got '%v'", data["id"])
	}
}

func TestE2E_RepliesCommand_MigratedContributions(t *testing.T) {
	requestPaths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		if r.URL.Path != "/v1/posts/prob-app/replies" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id": "reply-app-1", "post_id": "prob-app", "author_type": "agent", "author_id": "agent_claude",
					"body":        "Try using connection pooling: implement a pgx pool. Reduced connection overhead by 50%",
					"score":       1,
					"legacy_type": "approach",
					"legacy_id":   "app-1",
				},
				{
					"id": "reply-app-2", "post_id": "prob-app", "author_type": "human", "author_id": "user-2",
					"body":        "Optimize query structure",
					"legacy_type": "approach",
					"legacy_id":   "app-2",
				},
			},
			"meta": map[string]interface{}{"total": 2, "has_more": false},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"replies", "--api-url", server.URL + "/v1", "--api-key", "test_key", "prob-app"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// One call to the canonical replies endpoint, none to a legacy route
	if len(requestPaths) != 1 {
		t.Errorf("expected 1 API request, got %d: %v", len(requestPaths), requestPaths)
	}

	output := stdout.String()
	if !strings.Contains(output, "2 replies") {
		t.Error("output should count the replies")
	}
	if !strings.Contains(output, "Try using connection pooling") {
		t.Error("output should contain the migrated approach body")
	}
	if strings.Count(output, "migrated approach") != 2 {
		t.Errorf("output should label both migrated approaches, got: %s", output)
	}
}

func TestE2E_RepliesCommand_Threaded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/posts/q-ans/replies" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id": "reply-ans-1", "post_id": "q-ans", "author_type": "agent", "author_id": "agent_claude",
					"body":        "Use context.WithTimeout instead of time.After in the loop",
					"score":       2,
					"legacy_type": "answer",
				},
				{
					"id": "reply-ans-2", "post_id": "q-ans", "parent_reply_id": "reply-ans-1",
					"author_type": "human", "author_id": "user-3",
					"body": "That fixed it, thanks",
				},
			},
			"meta": map[string]interface{}{"total": 2, "has_more": false},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"replies", "--api-url", server.URL + "/v1", "--api-key", "test_key", "q-ans"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "time.After") {
		t.Error("output should contain the reply body")
	}
	if !strings.Contains(output, "migrated answer") {
		t.Error("output should label the migrated answer")
	}
	if !strings.Contains(output, "in reply to reply-ans-1") {
		t.Error("output should mark the threaded reply")
	}
}

func TestE2E_GetCommand_PointsToReplies(t *testing.T) {
	requestPaths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		if r.URL.Path != "/v1/posts/idea-resp" {
			http.NotFound(w, r)
			return
		}
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "idea-resp",
				Type:        "idea",
				Title:       "Idea with responses",
				Description: "Test idea",
				Tags:        []string{},
				Status:      "active",
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "Test",
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "--api-key", "test_key", "idea-resp"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// get reads the post only; its contributions are replies
	if len(requestPaths) != 1 || requestPaths[0] != "/v1/posts/idea-resp" {
		t.Errorf("expected exactly one request to /v1/posts/idea-resp, got %v", requestPaths)
	}

	output := stdout.String()
	if !strings.Contains(output, "Idea with responses") {
		t.Error("output should contain the post")
	}
	if !strings.Contains(output, "Replies: solvr replies idea-resp") {
		t.Errorf("output should point to the replies command, got: %s", output)
	}
}

func TestE2E_GetCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "NOT_FOUND",
				"message": "Post not found",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "nonexistent"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for not found post")
	}
}

func TestE2E_GetCommand_MissingID(t *testing.T) {
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"get"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when ID is missing")
	}
}

func TestE2E_RepliesCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/posts/q-json-inc/replies" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id": "reply-json-1", "post_id": "q-json-inc", "author_type": "agent", "author_id": "agent_claude",
					"body":        "JSON reply body",
					"upvotes":     5,
					"score":       5,
					"legacy_type": "answer",
				},
			},
			"meta": map[string]interface{}{"total": 1, "has_more": false},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"replies", "--api-url", server.URL + "/v1", "--api-key", "test_key", "--json", "q-json-inc"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify output is valid JSON with the replies page
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	replies, ok := result["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in JSON response")
	}
	if len(replies) != 1 {
		t.Errorf("expected 1 reply, got %d", len(replies))
	}
	if _, ok := result["meta"].(map[string]interface{}); !ok {
		t.Error("expected meta in JSON response")
	}
}

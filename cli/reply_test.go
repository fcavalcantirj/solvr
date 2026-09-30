package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// isolateHome points HOME at an empty directory so a test never reads the
// developer's ~/.solvr/config (a real API key and URL).
func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// unreachableAPI is used by tests that must fail before any request: a
// regression that still sends one cannot reach a real API.
const unreachableAPI = "http://127.0.0.1:9"

// TestReplyCommand_Exists verifies the reply command exists
func TestReplyCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	replyCmd, _, err := rootCmd.Find([]string{"reply"})
	if err != nil {
		t.Fatalf("reply command not found: %v", err)
	}
	if replyCmd == nil {
		t.Fatal("reply command is nil")
	}
	if replyCmd.Use != "reply <post_id>" {
		t.Errorf("expected Use to be 'reply <post_id>', got '%s'", replyCmd.Use)
	}
}

// TestReplyCommand_RequiresPostID verifies post_id argument is required
func TestReplyCommand_RequiresPostID(t *testing.T) {
	isolateHome(t)
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"reply", "--api-url", unreachableAPI})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when post_id not provided")
	}
	// Cobra returns "accepts 1 arg(s), received 0" for ExactArgs validation
	errStr := err.Error()
	if !strings.Contains(errStr, "post_id") && !strings.Contains(errStr, "required") && !strings.Contains(errStr, "accepts 1 arg") {
		t.Errorf("expected error about missing post_id, got: %s", errStr)
	}
}

// TestReplyCommand_RequiresBody verifies --body is required when not using editor
func TestReplyCommand_RequiresBody(t *testing.T) {
	isolateHome(t)
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"reply", "post_123", "--api-url", unreachableAPI})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --body not provided")
	}
	if !strings.Contains(err.Error(), "body") {
		t.Errorf("expected error to mention 'body', got: %s", err.Error())
	}
}

// TestReplyCommand_AcceptsBodyFlag verifies --body flag is accepted
func TestReplyCommand_AcceptsBodyFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "reply_123",
				"post_id":     "post_123",
				"body":        "This is the reply body.",
				"author_type": "human",
				"author_id":   "user_1",
				"upvotes":     0,
				"downvotes":   0,
				"score":       0,
				"created_at":  "2026-02-02T10:00:00Z",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "This is the reply body.",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestReplyCommand_SendsCorrectPayload verifies the canonical reply request:
// POST /posts/{id}/replies with only a body (no type, no legacy fields).
func TestReplyCommand_SendsCorrectPayload(t *testing.T) {
	var receivedBody map[string]interface{}
	var receivedPath string
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedMethod = r.Method
		json.NewDecoder(r.Body).Decode(&receivedBody)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":      "reply_123",
				"post_id": "post_123",
				"body":    "My detailed reply here",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "My detailed reply here",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedMethod != "POST" {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}

	// Every contribution is a reply of the post; no answer/approach route
	expectedPath := "/posts/post_123/replies"
	if receivedPath != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, receivedPath)
	}

	if receivedBody["body"] != "My detailed reply here" {
		t.Errorf("expected body 'My detailed reply here', got '%v'", receivedBody["body"])
	}
	if len(receivedBody) != 1 {
		t.Errorf("expected only the body field (no type, no parent), got %v", receivedBody)
	}
}

// TestReplyCommand_ThreadsUnderParentReply verifies --parent sends parent_reply_id
func TestReplyCommand_ThreadsUnderParentReply(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":              "reply_2",
				"post_id":         "post_123",
				"parent_reply_id": "reply_1",
				"body":            "Threaded reply",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "Threaded reply",
		"--parent", "reply_1",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedBody["parent_reply_id"] != "reply_1" {
		t.Errorf("expected parent_reply_id 'reply_1', got '%v'", receivedBody["parent_reply_id"])
	}
	if receivedBody["body"] != "Threaded reply" {
		t.Errorf("expected body 'Threaded reply', got '%v'", receivedBody["body"])
	}
	if !strings.Contains(buf.String(), "In reply to: reply_1") {
		t.Errorf("expected the parent reply in the output, got: %s", buf.String())
	}
}

// TestReplyCommand_AnswerCommandRemoved verifies the typed answer command is
// gone: an old invocation fails and sends nothing.
func TestReplyCommand_AnswerCommandRemoved(t *testing.T) {
	isolateHome(t)
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "Old answer",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected the removed answer command to fail")
	}
	if !strings.Contains(err.Error(), `unknown command "answer"`) {
		t.Errorf("expected unknown command error, got: %s", err.Error())
	}
	if called {
		t.Error("the removed answer command must not call the API")
	}
}

// TestReplyCommand_UsesAPIKey verifies Authorization header is set
func TestReplyCommand_UsesAPIKey(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "reply_123"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "Reply body",
		"--api-url", server.URL,
		"--api-key", "solvr_my_secret_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedAuth := "Bearer solvr_my_secret_key"
	if receivedAuth != expectedAuth {
		t.Errorf("expected Authorization '%s', got '%s'", expectedAuth, receivedAuth)
	}
}

// TestReplyCommand_DisplaysCreatedReply verifies success output format
func TestReplyCommand_DisplaysCreatedReply(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "reply_456",
				"post_id":     "post_789",
				"body":        "This is my reply.",
				"author_type": "human",
				"author_id":   "user_123",
				"upvotes":     0,
				"downvotes":   0,
				"score":       0,
				"created_at":  "2026-02-02T12:00:00Z",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_789",
		"--body", "This is my reply.",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		"Reply created successfully",
		"ID: reply_456",
		"Post ID: post_789",
		"This is my reply.",
		"Author: user_123 (human)",
		"solvr replies post_789",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("expected %q in output, got: %s", want, output)
		}
	}
	if strings.Contains(output, "In reply to") {
		t.Errorf("a top-level reply has no parent line, got: %s", output)
	}
}

// TestReplyCommand_JSONOutput verifies --json flag outputs raw JSON
func TestReplyCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":      "reply_123",
				"post_id": "post_123",
				"body":    "JSON output test",
				"upvotes": 0,
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "JSON output test",
		"--api-url", server.URL,
		"--api-key", "test_key",
		"--json",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'data' in JSON output, got: %s", output)
	}
	if data["body"] != "JSON output test" {
		t.Errorf("expected the reply body in JSON output, got: %v", data["body"])
	}
}

// TestReplyCommand_APIError verifies error handling for API errors
func TestReplyCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "VALIDATION_ERROR",
				"message": "reply body is too short",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "Too short",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for API error response")
	}

	if !strings.Contains(err.Error(), "reply body is too short") || !strings.Contains(err.Error(), "VALIDATION_ERROR") {
		t.Errorf("expected the API error code and message, got: %s", err.Error())
	}
}

// TestReplyCommand_Unauthorized verifies 401 error handling
func TestReplyCommand_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "UNAUTHORIZED",
				"message": "Invalid or missing API key",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "Reply body",
		"--api-url", server.URL,
		"--api-key", "invalid_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unauthorized response")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "401") && !strings.Contains(strings.ToLower(errStr), "unauthorized") && !strings.Contains(errStr, "API key") {
		t.Errorf("expected unauthorized error, got: %s", errStr)
	}
}

// TestReplyCommand_NotFound verifies 404 error handling (post not found)
func TestReplyCommand_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "NOT_FOUND",
				"message": "post not found",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "nonexistent_post",
		"--body", "Reply body",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for not found response")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "404") && !strings.Contains(strings.ToLower(errStr), "not found") {
		t.Errorf("expected not found error, got: %s", errStr)
	}
}

// TestReplyCommand_HelpText verifies help contains key information
func TestReplyCommand_HelpText(t *testing.T) {
	rootCmd := NewRootCmd()
	replyCmd, _, _ := rootCmd.Find([]string{"reply"})

	helpText := strings.ToLower(replyCmd.Long + replyCmd.Short)

	for _, want := range []string{"body", "reply", "--parent"} {
		if !strings.Contains(helpText, want) {
			t.Errorf("help should mention %q", want)
		}
	}
}

// TestReplyCommand_BodyShortFlag verifies -b short flag for body
func TestReplyCommand_BodyShortFlag(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "reply_123"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"-b", "Short flag body",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with -b flag: %v", err)
	}
	if receivedBody["body"] != "Short flag body" {
		t.Errorf("expected body 'Short flag body', got '%v'", receivedBody["body"])
	}
}

// TestReplyCommand_BodyPreserved verifies body whitespace is preserved
func TestReplyCommand_BodyPreserved(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "reply_123"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "  Body with whitespace  ",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Body is sent as written (leading/trailing spaces may matter in code)
	body, _ := receivedBody["body"].(string)
	if body != "  Body with whitespace  " {
		t.Errorf("expected body to be preserved, got '%s'", body)
	}
}

// TestReplyCommand_EmptyBodyRejected verifies empty body is rejected
func TestReplyCommand_EmptyBodyRejected(t *testing.T) {
	isolateHome(t)
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "",
		"--api-url", unreachableAPI,
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty body")
	}
	if !strings.Contains(err.Error(), "body") {
		t.Errorf("expected error to mention 'body', got: %s", err.Error())
	}
}

// TestReplyCommand_WhitespaceOnlyBodyRejected verifies whitespace-only body is rejected
func TestReplyCommand_WhitespaceOnlyBodyRejected(t *testing.T) {
	isolateHome(t)
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"reply", "post_123",
		"--body", "   \t\n   ",
		"--api-url", unreachableAPI,
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only body")
	}
	if !strings.Contains(err.Error(), "body") {
		t.Errorf("expected error to mention 'body', got: %s", err.Error())
	}
}

// Editor Mode Tests live in reply_editor_test.go

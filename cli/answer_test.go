package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestAnswerCommand_Exists verifies the answer command exists
func TestAnswerCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	answerCmd, _, err := rootCmd.Find([]string{"answer"})
	if err != nil {
		t.Fatalf("answer command not found: %v", err)
	}
	if answerCmd == nil {
		t.Fatal("answer command is nil")
	}
	if answerCmd.Use != "answer <post_id>" {
		t.Errorf("expected Use to be 'answer <post_id>', got '%s'", answerCmd.Use)
	}
}

// TestAnswerCommand_RequiresPostID verifies post_id argument is required
func TestAnswerCommand_RequiresPostID(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"answer"})

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

// TestAnswerCommand_RequiresContent verifies --content is required when not using editor
func TestAnswerCommand_RequiresContent(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{"answer", "post_123"})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when --content not provided")
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("expected error to mention 'content', got: %s", err.Error())
	}
}

// TestAnswerCommand_AcceptsContentFlag verifies --content flag is accepted
func TestAnswerCommand_AcceptsContentFlag(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return success response
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "answer_123",
				"question_id": "post_123",
				"content":     "This is the answer content.",
				"author_type": "human",
				"author_id":   "user_1",
				"upvotes":     0,
				"downvotes":   0,
				"is_accepted": false,
				"created_at":  "2026-02-02T10:00:00Z",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "This is the answer content.",
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

// TestAnswerCommand_SendsCorrectPayload verifies the correct JSON payload is sent
func TestAnswerCommand_SendsCorrectPayload(t *testing.T) {
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
				"id":          "answer_123",
				"question_id": "post_123",
				"content":     "My detailed answer here",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "My detailed answer here",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify HTTP method
	if receivedMethod != "POST" {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}

	// Verify path - should post to questions endpoint
	expectedPath := "/questions/post_123/answers"
	if receivedPath != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, receivedPath)
	}

	// Verify body
	if receivedBody["content"] != "My detailed answer here" {
		t.Errorf("expected content 'My detailed answer here', got '%v'", receivedBody["content"])
	}
}

// TestAnswerCommand_UsesAPIKey verifies Authorization header is set
func TestAnswerCommand_UsesAPIKey(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "answer_123"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "Answer content",
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

// TestAnswerCommand_DisplaysCreatedAnswer verifies success output format
func TestAnswerCommand_DisplaysCreatedAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "answer_456",
				"question_id": "question_789",
				"content":     "This is my answer.",
				"author_type": "human",
				"author_id":   "user_123",
				"upvotes":     0,
				"downvotes":   0,
				"is_accepted": false,
				"created_at":  "2026-02-02T12:00:00Z",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "question_789",
		"--content", "This is my answer.",
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

	// Should display success message
	if !strings.Contains(output, "Answer") && !strings.Contains(output, "created") {
		t.Errorf("expected success message in output, got: %s", output)
	}

	// Should show answer ID
	if !strings.Contains(output, "answer_456") {
		t.Errorf("expected answer ID in output, got: %s", output)
	}
}

// TestAnswerCommand_JSONOutput verifies --json flag outputs raw JSON
func TestAnswerCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"id":          "answer_123",
				"question_id": "post_123",
				"content":     "JSON output test",
				"upvotes":     0,
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "JSON output test",
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

	// Should be valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Should contain data
	if parsed["data"] == nil {
		t.Errorf("expected 'data' in JSON output")
	}
}

// TestAnswerCommand_APIError verifies error handling for API errors
func TestAnswerCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "VALIDATION_ERROR",
				"message": "Content is too short",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "Too short",
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

	if !strings.Contains(err.Error(), "Content is too short") && !strings.Contains(err.Error(), "API") {
		t.Errorf("expected API error message, got: %s", err.Error())
	}
}

// TestAnswerCommand_Unauthorized verifies 401 error handling
func TestAnswerCommand_Unauthorized(t *testing.T) {
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
		"answer", "post_123",
		"--content", "Answer content",
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

	// Should contain meaningful error
	errStr := err.Error()
	if !strings.Contains(errStr, "401") && !strings.Contains(strings.ToLower(errStr), "unauthorized") && !strings.Contains(errStr, "API key") {
		t.Errorf("expected unauthorized error, got: %s", errStr)
	}
}

// TestAnswerCommand_NotFound verifies 404 error handling (question not found)
func TestAnswerCommand_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "NOT_FOUND",
				"message": "Question not found",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "nonexistent_post",
		"--content", "Answer content",
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

	// Should contain meaningful error
	errStr := err.Error()
	if !strings.Contains(errStr, "404") && !strings.Contains(strings.ToLower(errStr), "not found") {
		t.Errorf("expected not found error, got: %s", errStr)
	}
}

// TestAnswerCommand_HelpText verifies help contains key information
func TestAnswerCommand_HelpText(t *testing.T) {
	rootCmd := NewRootCmd()
	answerCmd, _, _ := rootCmd.Find([]string{"answer"})

	helpText := answerCmd.Long + answerCmd.Short

	// Should mention content
	if !strings.Contains(strings.ToLower(helpText), "content") {
		t.Error("help should mention 'content'")
	}

	// Should mention answer/question
	if !strings.Contains(strings.ToLower(helpText), "answer") {
		t.Error("help should mention 'answer'")
	}
}

// TestAnswerCommand_ContentShortFlag verifies -c short flag for content
func TestAnswerCommand_ContentShortFlag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "answer_123"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"-c", "Short flag content",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error with -c flag: %v", err)
	}
}

// TestAnswerCommand_ContentTrimmed verifies content whitespace is preserved
func TestAnswerCommand_ContentTrimmed(t *testing.T) {
	var receivedBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{"id": "answer_123"},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "  Content with whitespace  ",
		"--api-url", server.URL,
		"--api-key", "test_key",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Content should be preserved (not trimmed - user might want leading/trailing spaces in code)
	// Actually, for answers, we might want to trim - but let's keep it as-is
	content := receivedBody["content"].(string)
	if content != "  Content with whitespace  " {
		t.Errorf("expected content to be preserved, got '%s'", content)
	}
}

// TestAnswerCommand_EmptyContentRejected verifies empty content is rejected
func TestAnswerCommand_EmptyContentRejected(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("expected error to mention 'content', got: %s", err.Error())
	}
}

// TestAnswerCommand_WhitespaceOnlyContentRejected verifies whitespace-only content is rejected
func TestAnswerCommand_WhitespaceOnlyContentRejected(t *testing.T) {
	rootCmd := NewRootCmd()
	rootCmd.SetArgs([]string{
		"answer", "post_123",
		"--content", "   \t\n   ",
	})

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for whitespace-only content")
	}
	if !strings.Contains(err.Error(), "content") {
		t.Errorf("expected error to mention 'content', got: %s", err.Error())
	}
}

// Editor Mode Tests have been moved to answer_editor_test.go

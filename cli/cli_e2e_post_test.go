package main

/**
 * E2E tests for CLI post command
 *
 * Per PRD line 5052-5057:
 * - E2E: CLI commands
 * - Test solvr post
 *
 * These tests verify the full CLI command execution flow for post.
 */

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ============================================================================
// E2E Test: solvr post command
// ============================================================================

func TestE2E_PostCommand_Problem(t *testing.T) {
	// Setup mock API server
	requestCount := 0
	var receivedBody CreatePostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/v1/posts" {
			t.Errorf("expected path /v1/posts, got %s", r.URL.Path)
		}

		// Verify content type
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", contentType)
		}

		// Parse request body
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		// Return success response
		response := CreatePostResponse{
			Data: CreatedPost{
				ID:        "prob-new-123",
				Type:      "problem",
				Title:     receivedBody.Title,
				Tags:      receivedBody.Tags,
				Status:    "open",
				CreatedAt: "2024-01-15T10:30:00Z",
			},
		}

		w.WriteHeader(http.StatusCreated)
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
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "Memory leak in async handler",
		"--description", "The async handler is leaking memory when processing large payloads...",
		"--tags", "go,async,memory",
		"problem",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify API was called
	if requestCount != 1 {
		t.Errorf("expected 1 API request, got %d", requestCount)
	}

	// Verify request body
	if receivedBody.Type != "problem" {
		t.Errorf("expected type 'problem', got '%s'", receivedBody.Type)
	}
	if receivedBody.Title != "Memory leak in async handler" {
		t.Errorf("expected title 'Memory leak in async handler', got '%s'", receivedBody.Title)
	}
	if len(receivedBody.Tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(receivedBody.Tags))
	}

	output := stdout.String()
	if !strings.Contains(output, "Post created successfully") {
		t.Error("output should confirm post creation")
	}
	if !strings.Contains(output, "prob-new-123") {
		t.Error("output should contain post ID")
	}
}

func TestE2E_PostCommand_Question(t *testing.T) {
	var receivedBody CreatePostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		response := CreatePostResponse{
			Data: CreatedPost{
				ID:     "q-new-456",
				Type:   "question",
				Title:  receivedBody.Title,
				Tags:   receivedBody.Tags,
				Status: "open",
			},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "How to implement retry logic?",
		"--description", "I need to implement exponential backoff for API calls...",
		"--tags", "go,retry",
		"question",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if receivedBody.Type != "question" {
		t.Errorf("expected type 'question', got '%s'", receivedBody.Type)
	}

	output := stdout.String()
	if !strings.Contains(output, "q-new-456") {
		t.Error("output should contain post ID")
	}
}

func TestE2E_PostCommand_Idea(t *testing.T) {
	var receivedBody CreatePostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		response := CreatePostResponse{
			Data: CreatedPost{
				ID:     "idea-new-789",
				Type:   "idea",
				Title:  receivedBody.Title,
				Tags:   receivedBody.Tags,
				Status: "open",
			},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "New caching strategy for search",
		"--description", "What if we implement a two-tier caching system...",
		"idea",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if receivedBody.Type != "idea" {
		t.Errorf("expected type 'idea', got '%s'", receivedBody.Type)
	}

	output := stdout.String()
	if !strings.Contains(output, "idea-new-789") {
		t.Error("output should contain post ID")
	}
}

func TestE2E_PostCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := CreatePostResponse{
			Data: CreatedPost{
				ID:        "post-json-out",
				Type:      "problem",
				Title:     "JSON output test",
				Tags:      []string{"test"},
				Status:    "open",
				CreatedAt: "2024-01-15T10:30:00Z",
			},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "JSON output test",
		"--description", "Testing JSON output",
		"--json",
		"problem",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify output is valid JSON
	var result CreatePostResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}

	if result.Data.ID != "post-json-out" {
		t.Errorf("expected ID 'post-json-out', got '%s'", result.Data.ID)
	}
}

func TestE2E_PostCommand_WithAPIKey(t *testing.T) {
	var receivedAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")

		response := CreatePostResponse{
			Data: CreatedPost{
				ID:     "post-auth",
				Type:   "question",
				Title:  "Auth test",
				Status: "open",
			},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--api-key", "solvr_test_key_123",
		"--title", "Auth test",
		"--description", "Testing authentication",
		"question",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if receivedAuthHeader != "Bearer solvr_test_key_123" {
		t.Errorf("expected auth header 'Bearer solvr_test_key_123', got '%s'", receivedAuthHeader)
	}
}

func TestE2E_PostCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "VALIDATION_ERROR",
				"message": "Title must be at least 10 characters",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "Short",
		"--description", "This should fail validation",
		"problem",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestE2E_PostCommand_InvalidType(t *testing.T) {
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{
		"post",
		"--title", "Test",
		"--description", "Test description",
		"invalid_type",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("error should mention invalid type, got: %v", err)
	}
}

func TestE2E_PostCommand_MissingTitle(t *testing.T) {
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{
		"post",
		"--description", "Test description",
		"problem",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing title")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("error should mention title, got: %v", err)
	}
}

func TestE2E_PostCommand_MissingDescription(t *testing.T) {
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{
		"post",
		"--title", "Test Title",
		"problem",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing description")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("error should mention description, got: %v", err)
	}
}

func TestE2E_PostCommand_MissingType(t *testing.T) {
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{
		"post",
		"--title", "Test Title",
		"--description", "Test description",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for missing type")
	}
	if !strings.Contains(err.Error(), "type is required") {
		t.Errorf("error should mention type requirement, got: %v", err)
	}
}

func TestE2E_PostCommand_TagsParsing(t *testing.T) {
	var receivedBody CreatePostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		response := CreatePostResponse{
			Data: CreatedPost{
				ID:     "post-tags",
				Type:   "problem",
				Title:  "Tags test",
				Tags:   receivedBody.Tags,
				Status: "open",
			},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "Tags test",
		"--description", "Testing tag parsing",
		"--tags", "go, async , memory,  performance",
		"problem",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify tags are properly trimmed
	expectedTags := []string{"go", "async", "memory", "performance"}
	if len(receivedBody.Tags) != len(expectedTags) {
		t.Errorf("expected %d tags, got %d", len(expectedTags), len(receivedBody.Tags))
	}
	for i, tag := range receivedBody.Tags {
		if tag != expectedTags[i] {
			t.Errorf("expected tag '%s', got '%s'", expectedTags[i], tag)
		}
	}
}

func TestE2E_PostCommand_NoTags(t *testing.T) {
	var receivedBody CreatePostRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)

		response := CreatePostResponse{
			Data: CreatedPost{
				ID:     "post-no-tags",
				Type:   "question",
				Title:  "No tags test",
				Tags:   nil,
				Status: "open",
			},
		}

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "No tags test",
		"--description", "Testing without tags",
		"question",
	})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Tags should be nil or empty
	if len(receivedBody.Tags) != 0 {
		t.Errorf("expected no tags, got %v", receivedBody.Tags)
	}
}

func TestE2E_PostCommand_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "INTERNAL_ERROR",
				"message": "Database connection failed",
			},
		})
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{
		"post",
		"--api-url", server.URL + "/v1",
		"--title", "Server error test",
		"--description", "This should trigger server error",
		"problem",
	})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error for server error")
	}
}

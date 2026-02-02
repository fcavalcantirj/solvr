package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	postCmd, _, err := rootCmd.Find([]string{"post"})
	if err != nil {
		t.Fatalf("post command not found: %v", err)
	}
	if postCmd == nil {
		t.Fatal("post command is nil")
	}
}

func TestPostCommand_RequiresType(t *testing.T) {
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"post"})

	err := rootCmd.Execute()
	// Should fail because no type provided
	if err == nil {
		t.Error("expected error when no type provided")
	}
}

func TestPostCommand_RequiresValidType(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Should not be called for invalid type
		t.Error("API should not be called for invalid type")
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Title")
	postCmd.Flags().Set("description", "Test description that is long enough to meet the minimum requirement")
	postCmd.SetArgs([]string{"invalid_type"})

	err := postCmd.Execute()
	if err == nil {
		t.Error("expected error for invalid type")
	}
	if !strings.Contains(err.Error(), "invalid type") {
		t.Errorf("expected error message to mention 'invalid type', got: %v", err)
	}
}

func TestPostCommand_AcceptsProblemType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "problem",
				"title": "Test Problem",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Problem")
	postCmd.Flags().Set("description", "This is a test description for the problem that is long enough")
	postCmd.SetArgs([]string{"problem"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed for problem type: %v", err)
	}
}

func TestPostCommand_AcceptsQuestionType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-456",
				"type":  "question",
				"title": "Test Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Question")
	postCmd.Flags().Set("description", "This is a test description for the question that is long enough")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed for question type: %v", err)
	}
}

func TestPostCommand_AcceptsIdeaType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-789",
				"type":  "idea",
				"title": "Test Idea",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Idea")
	postCmd.Flags().Set("description", "This is a test description for the idea that is long enough")
	postCmd.SetArgs([]string{"idea"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed for idea type: %v", err)
	}
}

func TestPostCommand_RequiresTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called without title")
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("description", "This is a test description for the post that is long enough")
	// No title set
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err == nil {
		t.Error("expected error when title not provided")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("expected error message to mention 'title', got: %v", err)
	}
}

func TestPostCommand_RequiresDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API should not be called without description")
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Title")
	// No description set
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err == nil {
		t.Error("expected error when description not provided")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("expected error message to mention 'description', got: %v", err)
	}
}

func TestPostCommand_SendsCorrectPayload(t *testing.T) {
	var receivedPayload map[string]interface{}
	var receivedMethod string
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "My Test Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed: %v", err)
	}

	// Check HTTP method
	if receivedMethod != "POST" {
		t.Errorf("expected POST method, got %s", receivedMethod)
	}

	// Check path - should post to /posts
	if receivedPath != "/posts" {
		t.Errorf("expected path '/posts', got '%s'", receivedPath)
	}

	// Check payload
	if receivedPayload["type"] != "question" {
		t.Errorf("expected type 'question', got '%v'", receivedPayload["type"])
	}
	if receivedPayload["title"] != "My Test Question" {
		t.Errorf("expected title 'My Test Question', got '%v'", receivedPayload["title"])
	}
	if receivedPayload["description"] != "This is a detailed description of my question that is long enough" {
		t.Errorf("expected description in payload, got '%v'", receivedPayload["description"])
	}
}

func TestPostCommand_SupportsTags(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Question with Tags")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.Flags().Set("tags", "go,async,postgres")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed: %v", err)
	}

	// Check tags in payload
	tags, ok := receivedPayload["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags to be an array, got %T", receivedPayload["tags"])
	}
	if len(tags) != 3 {
		t.Errorf("expected 3 tags, got %d", len(tags))
	}
	expectedTags := []string{"go", "async", "postgres"}
	for i, tag := range tags {
		if tag != expectedTags[i] {
			t.Errorf("expected tag '%s', got '%v'", expectedTags[i], tag)
		}
	}
}

func TestPostCommand_UsesAPIKey(t *testing.T) {
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("api-key", "solvr_test_key")
	postCmd.Flags().Set("title", "Test Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed: %v", err)
	}

	if receivedAuthHeader != "Bearer solvr_test_key" {
		t.Errorf("expected Authorization header 'Bearer solvr_test_key', got '%s'", receivedAuthHeader)
	}
}

func TestPostCommand_DisplaysCreatedPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-abc123",
				"type":  "question",
				"title": "Created Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Created Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed: %v", err)
	}

	output := buf.String()
	// Should display success message with post ID
	if !strings.Contains(output, "post-abc123") {
		t.Errorf("output should contain the post ID, got: %s", output)
	}
	if !strings.Contains(output, "Created") || !strings.Contains(output, "created") {
		t.Errorf("output should indicate success, got: %s", output)
	}
}

func TestPostCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.Flags().Set("json", "true")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed: %v", err)
	}

	output := buf.String()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON, got error: %v\nOutput was: %s", err, output)
	}

	// Should contain data field
	if _, ok := result["data"]; !ok {
		t.Error("JSON output should contain 'data' field")
	}
}

func TestPostCommand_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"title too short"}}`))
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err == nil {
		t.Error("expected error when API returns error")
	}
}

func TestPostCommand_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"UNAUTHORIZED","message":"missing or invalid token"}}`))
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err == nil {
		t.Error("expected error when API returns 401")
	}
	if !strings.Contains(err.Error(), "UNAUTHORIZED") && !strings.Contains(err.Error(), "unauthorized") && !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected error to mention unauthorized, got: %v", err)
	}
}

func TestPostCommand_HelpText(t *testing.T) {
	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetArgs([]string{"--help"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// Check that help contains key information
	if !strings.Contains(output, "post") {
		t.Error("help should mention 'post'")
	}
	if !strings.Contains(output, "--title") {
		t.Error("help should mention '--title' flag")
	}
	if !strings.Contains(output, "--description") {
		t.Error("help should mention '--description' flag")
	}
	if !strings.Contains(output, "--tags") {
		t.Error("help should mention '--tags' flag")
	}
	if !strings.Contains(output, "problem") && !strings.Contains(output, "question") && !strings.Contains(output, "idea") {
		t.Error("help should mention valid types")
	}
}

func TestPostCommand_TagsOptional(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	// No tags set
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command should succeed without tags: %v", err)
	}
}

func TestPostCommand_TrimsTagWhitespace(t *testing.T) {
	var receivedPayload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedPayload)

		response := map[string]interface{}{
			"data": map[string]interface{}{
				"id":    "post-123",
				"type":  "question",
				"title": "Test Question",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	postCmd := NewPostCmd()
	buf := new(bytes.Buffer)
	postCmd.SetOut(buf)
	postCmd.SetErr(buf)
	postCmd.Flags().Set("api-url", server.URL)
	postCmd.Flags().Set("title", "Test Question")
	postCmd.Flags().Set("description", "This is a detailed description of my question that is long enough")
	postCmd.Flags().Set("tags", "  go , async ,  postgres  ")
	postCmd.SetArgs([]string{"question"})

	err := postCmd.Execute()
	if err != nil {
		t.Fatalf("post command failed: %v", err)
	}

	// Check tags are trimmed
	tags, ok := receivedPayload["tags"].([]interface{})
	if !ok {
		t.Fatalf("expected tags to be an array, got %T", receivedPayload["tags"])
	}
	expectedTags := []string{"go", "async", "postgres"}
	for i, tag := range tags {
		if tag != expectedTags[i] {
			t.Errorf("expected trimmed tag '%s', got '%v'", expectedTags[i], tag)
		}
	}
}

func TestPostCommand_PostsToCorrectEndpoint(t *testing.T) {
	tests := []struct {
		postType     string
		expectedPath string
	}{
		{"problem", "/posts"},
		{"question", "/posts"},
		{"idea", "/posts"},
	}

	for _, tt := range tests {
		t.Run(tt.postType, func(t *testing.T) {
			var receivedPath string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path

				response := map[string]interface{}{
					"data": map[string]interface{}{
						"id":    "post-123",
						"type":  tt.postType,
						"title": "Test",
					},
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusCreated)
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			postCmd := NewPostCmd()
			buf := new(bytes.Buffer)
			postCmd.SetOut(buf)
			postCmd.SetErr(buf)
			postCmd.Flags().Set("api-url", server.URL)
			postCmd.Flags().Set("title", "Test Post")
			postCmd.Flags().Set("description", "This is a detailed description of my post that is long enough")
			postCmd.SetArgs([]string{tt.postType})

			err := postCmd.Execute()
			if err != nil {
				t.Fatalf("post command failed for type %s: %v", tt.postType, err)
			}

			if receivedPath != tt.expectedPath {
				t.Errorf("expected path '%s', got '%s'", tt.expectedPath, receivedPath)
			}
		})
	}
}

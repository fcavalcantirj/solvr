package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	getCmd, _, err := rootCmd.Find([]string{"get"})
	if err != nil {
		t.Fatalf("get command not found: %v", err)
	}
	if getCmd == nil {
		t.Fatal("get command is nil")
	}
}

func TestGetCommand_RequiresID(t *testing.T) {
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"get"})

	err := rootCmd.Execute()
	// Should fail because no ID provided
	if err == nil {
		t.Error("expected error when no ID provided")
	}
}

func TestGetCommand_CallsAPI(t *testing.T) {
	// Create mock API server
	apiCalled := false
	var receivedID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		// Extract ID from path: /posts/post-123
		receivedID = r.URL.Path[len("/posts/"):]

		// Return mock response
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-123",
				Type:        "question",
				Title:       "How to fix async bugs?",
				Description: "I have an async bug in my code that I need help with...",
				Tags:        []string{"go", "async"},
				Status:      "open",
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "John",
				},
				Upvotes:   10,
				Downvotes: 2,
				VoteScore: 8,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create command with mock API URL
	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)

	// Override API URL for testing
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	if !apiCalled {
		t.Error("API was not called")
	}

	if receivedID != "post-123" {
		t.Errorf("expected ID 'post-123', got '%s'", receivedID)
	}
}

func TestGetCommand_DisplaysDetails(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-123",
				Type:        "question",
				Title:       "How to fix async bugs?",
				Description: "I have an async bug in my code that I need help with...",
				Tags:        []string{"go", "async"},
				Status:      "open",
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "John",
				},
				Upvotes:   10,
				Downvotes: 2,
				VoteScore: 8,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains key information
	if !bytes.Contains([]byte(output), []byte("How to fix async bugs?")) {
		t.Error("output should contain the post title")
	}
	if !bytes.Contains([]byte(output), []byte("question")) {
		t.Error("output should contain the post type")
	}
	if !bytes.Contains([]byte(output), []byte("John")) {
		t.Error("output should contain the author name")
	}
}

func TestGetCommand_NotFound(t *testing.T) {
	// Create mock API server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"post not found"}}`))
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"nonexistent"})

	err := getCmd.Execute()
	// Should return an error
	if err == nil {
		t.Error("expected error when post not found")
	}
}

func TestGetCommand_APIError(t *testing.T) {
	// Create mock API server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"server error"}}`))
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	// Should return an error
	if err == nil {
		t.Error("expected error when API returns error")
	}
}

func TestGetCommand_UsesAPIKey(t *testing.T) {
	// Create mock API server that checks for auth header
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")

		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-123",
				Type:        "question",
				Title:       "Test Post",
				Description: "Test description...",
				Status:      "open",
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

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.Flags().Set("api-key", "solvr_test123")
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	if receivedAuthHeader != "Bearer solvr_test123" {
		t.Errorf("expected Authorization header 'Bearer solvr_test123', got '%s'", receivedAuthHeader)
	}
}

func TestGetCommand_HelpText(t *testing.T) {
	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetArgs([]string{"--help"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// Check that help contains key information
	if !bytes.Contains([]byte(output), []byte("get")) {
		t.Error("help should mention 'get'")
	}
	if !bytes.Contains([]byte(output), []byte("id")) {
		t.Error("help should mention 'id'")
	}
}

func TestGetCommand_JSONFlag(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-123",
				Type:        "question",
				Title:       "How to fix async bugs?",
				Description: "I have an async bug in my code...",
				Tags:        []string{"go", "async"},
				Status:      "open",
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "John",
				},
				Upvotes:   10,
				Downvotes: 2,
				VoteScore: 8,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.Flags().Set("json", "true")
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()

	// Output should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON, got error: %v\nOutput was: %s", err, output)
	}

	// Should contain data field
	if _, ok := result["data"]; !ok {
		t.Error("JSON output should contain 'data' field")
	}
}

func TestGetCommand_DisplaysProblemDetails(t *testing.T) {
	// Create mock API server that returns a problem with success criteria
	weight := 3
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetAPIResponse{
			Data: PostDetail{
				ID:              "prob-123",
				Type:            "problem",
				Title:           "Race condition in database",
				Description:     "We have a race condition when multiple goroutines access the database...",
				Tags:            []string{"go", "database", "concurrency"},
				Status:          "in_progress",
				SuccessCriteria: []string{"No race conditions", "Tests pass"},
				Weight:          &weight,
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "Jane",
				},
				Upvotes:   15,
				Downvotes: 1,
				VoteScore: 14,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"prob-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains problem-specific information
	if !bytes.Contains([]byte(output), []byte("Race condition in database")) {
		t.Error("output should contain the problem title")
	}
	if !bytes.Contains([]byte(output), []byte("problem")) {
		t.Error("output should contain the post type")
	}
	if !bytes.Contains([]byte(output), []byte("Success Criteria")) || !bytes.Contains([]byte(output), []byte("No race conditions")) {
		t.Error("output should contain success criteria")
	}
	if !bytes.Contains([]byte(output), []byte("Weight")) {
		t.Error("output should contain weight for problems")
	}
}

func TestGetCommand_DisplaysVotes(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-123",
				Type:        "question",
				Title:       "Test question",
				Description: "Test description...",
				Status:      "open",
				Author: AuthorInfo{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "Test",
				},
				Upvotes:   25,
				Downvotes: 5,
				VoteScore: 20,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains vote information
	if !bytes.Contains([]byte(output), []byte("20")) { // vote score
		t.Error("output should contain vote score")
	}
}

func TestGetCommand_DisplaysDescription(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := GetAPIResponse{
			Data: PostDetail{
				ID:          "post-123",
				Type:        "question",
				Title:       "Test question",
				Description: "This is a detailed description of the problem that needs to be shown in the output.",
				Status:      "open",
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

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.SetArgs([]string{"post-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains the description
	if !bytes.Contains([]byte(output), []byte("detailed description")) {
		t.Error("output should contain the description")
	}
}

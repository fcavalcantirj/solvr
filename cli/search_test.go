package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchCommand_Exists(t *testing.T) {
	rootCmd := NewRootCmd()
	searchCmd, _, err := rootCmd.Find([]string{"search"})
	if err != nil {
		t.Fatalf("search command not found: %v", err)
	}
	if searchCmd == nil {
		t.Fatal("search command is nil")
	}
}

func TestSearchCommand_RequiresQuery(t *testing.T) {
	rootCmd := NewRootCmd()
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"search"})

	err := rootCmd.Execute()
	// Should fail because no query provided
	if err == nil {
		t.Error("expected error when no query provided")
	}
}

func TestSearchCommand_CallsAPI(t *testing.T) {
	// Create mock API server
	apiCalled := false
	var receivedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalled = true
		receivedQuery = r.URL.Query().Get("q")

		// Return mock response
		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-123",
					Type:    "question",
					Title:   "How to fix async bugs?",
					Snippet: "I have an <mark>async</mark> bug...",
					Tags:    []string{"go", "async"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "John",
					},
					Score:        0.95,
					Votes:        10,
					AnswersCount: 2,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "async bug",
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create command with mock API URL
	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)

	// Override API URL for testing
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.SetArgs([]string{"async bug"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	if !apiCalled {
		t.Error("API was not called")
	}

	if receivedQuery != "async bug" {
		t.Errorf("expected query 'async bug', got '%s'", receivedQuery)
	}
}

func TestSearchCommand_DisplaysResults(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-123",
					Type:    "question",
					Title:   "How to fix async bugs?",
					Snippet: "I have an async bug in my code...",
					Tags:    []string{"go", "async"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "John",
					},
					Score:        0.95,
					Votes:        10,
					AnswersCount: 2,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "async bug",
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  15,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.SetArgs([]string{"async bug"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := buf.String()

	// Check that output contains key information
	if !bytes.Contains([]byte(output), []byte("How to fix async bugs?")) {
		t.Error("output should contain the post title")
	}
	if !bytes.Contains([]byte(output), []byte("question")) {
		t.Error("output should contain the post type")
	}
}

func TestSearchCommand_NoResults(t *testing.T) {
	// Create mock API server that returns no results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{
				Query:   "nonexistent query",
				Total:   0,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.SetArgs([]string{"nonexistent query"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("No results found")) {
		t.Error("output should indicate no results found")
	}
}

func TestSearchCommand_APIError(t *testing.T) {
	// Create mock API server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"server error"}}`))
	}))
	defer server.Close()

	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.SetArgs([]string{"test query"})

	err := searchCmd.Execute()
	// Should return an error
	if err == nil {
		t.Error("expected error when API returns error")
	}
}

func TestSearchCommand_UsesAPIKey(t *testing.T) {
	// Create mock API server that checks for auth header
	var receivedAuthHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")

		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{
				Query:   "test",
				Total:   0,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.Flags().Set("api-key", "solvr_test123")
	searchCmd.SetArgs([]string{"test"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	if receivedAuthHeader != "Bearer solvr_test123" {
		t.Errorf("expected Authorization header 'Bearer solvr_test123', got '%s'", receivedAuthHeader)
	}
}

func TestSearchCommand_HelpText(t *testing.T) {
	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetArgs([]string{"--help"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// Check that help contains key information
	if !bytes.Contains([]byte(output), []byte("search")) {
		t.Error("help should mention 'search'")
	}
	if !bytes.Contains([]byte(output), []byte("query")) {
		t.Error("help should mention 'query'")
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"plain text", "plain text"},
		{"<mark>highlighted</mark>", "highlighted"},
		{"I have an <mark>async</mark> bug", "I have an async bug"},
		{"<b>bold</b> and <i>italic</i>", "bold and italic"},
		{"no tags here", "no tags here"},
		{"<>empty</>", "empty"},
	}

	for _, tt := range tests {
		result := stripHTMLTags(tt.input)
		if result != tt.expected {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

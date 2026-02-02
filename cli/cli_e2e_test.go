package main

/**
 * E2E tests for CLI search command
 *
 * Per PRD line 5052-5057:
 * - E2E: CLI commands
 * - Test solvr search
 *
 * These tests verify the full CLI command execution flow for search.
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
// E2E Test: solvr search command
// ============================================================================

func TestE2E_SearchCommand_FullFlow(t *testing.T) {
	// Setup mock API server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify request path (search.go appends /search to base URL)
		if r.URL.Path != "/search" {
			t.Errorf("expected path /search, got %s", r.URL.Path)
		}

		// Verify query parameter
		query := r.URL.Query().Get("q")
		if query != "async postgres error" {
			t.Errorf("expected query 'async postgres error', got '%s'", query)
		}

		// Return realistic API response
		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-e2e-001",
					Type:    "problem",
					Title:   "Race condition in async PostgreSQL queries",
					Snippet: "Multiple goroutines accessing the same <mark>connection</mark> pool...",
					Tags:    []string{"postgresql", "async", "go"},
					Status:  "solved",
					Author: AuthorInfo{
						ID:          "agent_claude",
						Type:        "agent",
						DisplayName: "Claude Assistant",
					},
					Score:        0.92,
					Votes:        42,
					AnswersCount: 3,
					CreatedAt:    time.Now().Add(-24 * time.Hour),
				},
				{
					ID:      "post-e2e-002",
					Type:    "question",
					Title:   "How to handle async errors in PostgreSQL?",
					Snippet: "I'm getting <mark>timeout errors</mark> when running concurrent queries...",
					Tags:    []string{"postgresql", "error-handling"},
					Status:  "answered",
					Author: AuthorInfo{
						ID:          "user-123",
						Type:        "human",
						DisplayName: "John Developer",
					},
					Score:        0.87,
					Votes:        15,
					AnswersCount: 2,
					CreatedAt:    time.Now().Add(-48 * time.Hour),
				},
			},
			Meta: SearchMeta{
				Query:   "async postgres error",
				Total:   2,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  23,
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
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "async postgres error"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify API was called
	if requestCount != 1 {
		t.Errorf("expected 1 API request, got %d", requestCount)
	}

	output := stdout.String()

	// Verify output contains search results
	if !strings.Contains(output, "Race condition in async PostgreSQL queries") {
		t.Error("output should contain first result title")
	}
	if !strings.Contains(output, "How to handle async errors in PostgreSQL?") {
		t.Error("output should contain second result title")
	}
	if !strings.Contains(output, "problem") {
		t.Error("output should show post type 'problem'")
	}
	if !strings.Contains(output, "question") {
		t.Error("output should show post type 'question'")
	}
}

func TestE2E_SearchCommand_WithTypeFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify type filter is passed
		typeFilter := r.URL.Query().Get("type")
		if typeFilter != "question" {
			t.Errorf("expected type filter 'question', got '%s'", typeFilter)
		}

		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-e2e-003",
					Type:    "question",
					Title:   "Best practices for error handling?",
					Snippet: "Looking for guidance on <mark>error</mark> patterns...",
					Tags:    []string{"best-practices"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-456",
						Type:        "human",
						DisplayName: "Jane Dev",
					},
					Score:        0.75,
					Votes:        5,
					AnswersCount: 0,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "error handling",
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  18,
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
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "--type", "question", "error handling"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Best practices for error handling?") {
		t.Error("output should contain filtered result")
	}
}

func TestE2E_SearchCommand_WithLimitFilter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		perPage := r.URL.Query().Get("per_page")
		if perPage != "5" {
			t.Errorf("expected per_page '5', got '%s'", perPage)
		}

		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{
				Query:   "test",
				Total:   0,
				Page:    1,
				PerPage: 5,
				HasMore: false,
				TookMs:  5,
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
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "--limit", "5", "test"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No results found") {
		t.Error("output should indicate no results")
	}
}

func TestE2E_SearchCommand_JSONOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-e2e-json",
					Type:    "idea",
					Title:   "JSON test idea",
					Snippet: "Test snippet",
					Tags:    []string{"test"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "agent_test",
						Type:        "agent",
						DisplayName: "Test Agent",
					},
					Score:        0.99,
					Votes:        100,
					AnswersCount: 0,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "json test",
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
				TookMs:  10,
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
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "--json", "json test"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify output is valid JSON
	var result SearchAPIResponse
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, stdout.String())
	}

	if len(result.Data) != 1 {
		t.Errorf("expected 1 result, got %d", len(result.Data))
	}
	if result.Data[0].ID != "post-e2e-json" {
		t.Errorf("expected ID 'post-e2e-json', got '%s'", result.Data[0].ID)
	}
}

func TestE2E_SearchCommand_APIError(t *testing.T) {
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
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "test query"})

	err := rootCmd.Execute()
	// Should return error for API failure
	if err == nil {
		t.Error("expected error for API failure")
	}
}

func TestE2E_SearchCommand_WithAPIKey(t *testing.T) {
	var receivedAuthHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")

		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{Query: "test", Total: 0, Page: 1, PerPage: 20},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	rootCmd.SetOut(new(bytes.Buffer))
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "--api-key", "solvr_test_key_123", "test"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	if receivedAuthHeader != "Bearer solvr_test_key_123" {
		t.Errorf("expected auth header 'Bearer solvr_test_key_123', got '%s'", receivedAuthHeader)
	}
}

func TestE2E_SearchCommand_NoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{
				Query:   "nonexistent term xyz",
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

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "nonexistent term xyz"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "No results found") {
		t.Error("output should indicate no results found")
	}
}

func TestE2E_SearchCommand_MissingQuery(t *testing.T) {
	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs([]string{"search"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when query is missing")
	}
}

func TestE2E_SearchCommand_MultipleResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-1",
					Type:    "problem",
					Title:   "First problem",
					Snippet: "First snippet",
					Tags:    []string{"tag1"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "User One",
					},
					Score:        0.95,
					Votes:        50,
					AnswersCount: 5,
					CreatedAt:    time.Now(),
				},
				{
					ID:      "post-2",
					Type:    "question",
					Title:   "Second question",
					Snippet: "Second snippet",
					Tags:    []string{"tag2"},
					Status:  "answered",
					Author: AuthorInfo{
						ID:          "agent_claude",
						Type:        "agent",
						DisplayName: "Claude",
					},
					Score:        0.85,
					Votes:        30,
					AnswersCount: 3,
					CreatedAt:    time.Now(),
				},
				{
					ID:      "post-3",
					Type:    "idea",
					Title:   "Third idea",
					Snippet: "Third snippet",
					Tags:    []string{"tag3"},
					Status:  "exploring",
					Author: AuthorInfo{
						ID:          "user-2",
						Type:        "human",
						DisplayName: "User Two",
					},
					Score:        0.75,
					Votes:        10,
					AnswersCount: 1,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "multi",
				Total:   3,
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

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "multi"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "First problem") {
		t.Error("output should contain first result")
	}
	if !strings.Contains(output, "Second question") {
		t.Error("output should contain second result")
	}
	if !strings.Contains(output, "Third idea") {
		t.Error("output should contain third result")
	}
	if !strings.Contains(output, "Found 3 result(s)") {
		t.Error("output should show result count")
	}
}

func TestE2E_SearchCommand_HasMore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-1",
					Type:    "problem",
					Title:   "Result one",
					Snippet: "Snippet",
					Tags:    []string{},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Test",
					},
					Score:        0.9,
					Votes:        1,
					AnswersCount: 0,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "test",
				Total:   50,
				Page:    1,
				PerPage: 20,
				HasMore: true,
				TookMs:  10,
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
	rootCmd.SetArgs([]string{"search", "--api-url", server.URL, "test"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "More results available") {
		t.Error("output should indicate more results available")
	}
}

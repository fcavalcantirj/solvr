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

func TestSearchCommand_JSONFlag(t *testing.T) {
	// Create mock API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.Flags().Set("json", "true")
	searchCmd.SetArgs([]string{"async bug"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
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

	// Should contain meta field
	if _, ok := result["meta"]; !ok {
		t.Error("JSON output should contain 'meta' field")
	}
}

func TestSearchCommand_JSONFlagWithNoResults(t *testing.T) {
	// Create mock API server that returns no results
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{
				Query:   "nonexistent",
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
	searchCmd.Flags().Set("json", "true")
	searchCmd.SetArgs([]string{"nonexistent"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	output := buf.String()

	// Output should still be valid JSON even with no results
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Errorf("output should be valid JSON, got error: %v", err)
	}

	// data should be an empty array
	data, ok := result["data"].([]interface{})
	if !ok {
		t.Error("data should be an array")
	}
	if len(data) != 0 {
		t.Error("data should be empty")
	}
}

func TestSearchCommand_TypeFlag(t *testing.T) {
	// Create mock API server that checks for type parameter
	var receivedType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedType = r.URL.Query().Get("type")

		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-123",
					Type:    "problem",
					Title:   "A problem post",
					Snippet: "This is a problem...",
					Tags:    []string{"go"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "John",
					},
					Score:        0.90,
					Votes:        5,
					AnswersCount: 0,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "test",
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

	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetErr(buf)
	searchCmd.Flags().Set("api-url", server.URL)
	searchCmd.Flags().Set("type", "problem")
	searchCmd.SetArgs([]string{"test"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	if receivedType != "problem" {
		t.Errorf("expected type 'problem', got '%s'", receivedType)
	}
}

func TestSearchCommand_TypeFlagAllValues(t *testing.T) {
	// Test all valid type values
	validTypes := []string{"problem", "question", "idea", "all"}

	for _, typeVal := range validTypes {
		t.Run("type="+typeVal, func(t *testing.T) {
			var receivedType string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedType = r.URL.Query().Get("type")
				response := SearchAPIResponse{
					Data: []SearchResult{},
					Meta: SearchMeta{Query: "test", Total: 0},
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
			searchCmd.Flags().Set("type", typeVal)
			searchCmd.SetArgs([]string{"test"})

			err := searchCmd.Execute()
			if err != nil {
				t.Fatalf("search command failed for type %s: %v", typeVal, err)
			}

			if receivedType != typeVal {
				t.Errorf("expected type '%s', got '%s'", typeVal, receivedType)
			}
		})
	}
}

func TestSearchCommand_TypeFlagInHelpText(t *testing.T) {
	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetArgs([]string{"--help"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// Check that help contains --type flag information
	if !bytes.Contains([]byte(output), []byte("--type")) {
		t.Error("help should mention '--type' flag")
	}
}

func TestSearchCommand_LimitFlag(t *testing.T) {
	// Create mock API server that checks for per_page parameter
	var receivedLimit string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLimit = r.URL.Query().Get("per_page")

		response := SearchAPIResponse{
			Data: []SearchResult{
				{
					ID:      "post-123",
					Type:    "question",
					Title:   "Result 1",
					Snippet: "This is result 1...",
					Tags:    []string{"go"},
					Status:  "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "John",
					},
					Score:        0.90,
					Votes:        5,
					AnswersCount: 0,
					CreatedAt:    time.Now(),
				},
			},
			Meta: SearchMeta{
				Query:   "test",
				Total:   10,
				Page:    1,
				PerPage: 5,
				HasMore: true,
				TookMs:  10,
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
	searchCmd.Flags().Set("limit", "5")
	searchCmd.SetArgs([]string{"test"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	if receivedLimit != "5" {
		t.Errorf("expected per_page '5', got '%s'", receivedLimit)
	}
}

func TestSearchCommand_LimitFlagDefaultNotSent(t *testing.T) {
	// Create mock API server that checks that per_page is NOT sent when limit not specified
	var receivedLimit string
	var hasPerPageParam bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedLimit = r.URL.Query().Get("per_page")
		hasPerPageParam = r.URL.Query().Has("per_page")

		response := SearchAPIResponse{
			Data: []SearchResult{},
			Meta: SearchMeta{Query: "test", Total: 0},
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
	// Don't set --limit flag
	searchCmd.SetArgs([]string{"test"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("search command failed: %v", err)
	}

	// per_page should NOT be sent when limit is not specified (use API default)
	if hasPerPageParam {
		t.Errorf("per_page should not be sent when --limit not specified, but got '%s'", receivedLimit)
	}
}

func TestSearchCommand_LimitFlagInHelpText(t *testing.T) {
	searchCmd := NewSearchCmd()
	buf := new(bytes.Buffer)
	searchCmd.SetOut(buf)
	searchCmd.SetArgs([]string{"--help"})

	err := searchCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// Check that help contains --limit flag information
	if !bytes.Contains([]byte(output), []byte("--limit")) {
		t.Error("help should mention '--limit' flag")
	}
}

func TestSearchCommand_LimitFlagDifferentValues(t *testing.T) {
	// Test different limit values
	limitValues := []string{"1", "10", "50"}

	for _, limitVal := range limitValues {
		t.Run("limit="+limitVal, func(t *testing.T) {
			var receivedLimit string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedLimit = r.URL.Query().Get("per_page")
				response := SearchAPIResponse{
					Data: []SearchResult{},
					Meta: SearchMeta{Query: "test", Total: 0},
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
			searchCmd.Flags().Set("limit", limitVal)
			searchCmd.SetArgs([]string{"test"})

			err := searchCmd.Execute()
			if err != nil {
				t.Fatalf("search command failed for limit %s: %v", limitVal, err)
			}

			if receivedLimit != limitVal {
				t.Errorf("expected limit '%s', got '%s'", limitVal, receivedLimit)
			}
		})
	}
}

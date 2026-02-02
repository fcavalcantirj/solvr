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

func TestE2E_GetCommand_WithApproaches(t *testing.T) {
	requestPaths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)

		if r.URL.Path == "/v1/posts/prob-app" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "prob-app",
					Type:        "problem",
					Title:       "Problem with approaches",
					Description: "Test problem",
					Tags:        []string{},
					Status:      "working",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Test",
					},
					Upvotes:   1,
					Downvotes: 0,
					VoteScore: 1,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/v1/problems/prob-app/approaches" {
			response := ApproachesAPIResponse{
				Data: []ApproachDetail{
					{
						ID:        "app-1",
						ProblemID: "prob-app",
						Angle:     "Try using connection pooling",
						Method:    "Implement pgx pool",
						Status:    "succeeded",
						Outcome:   "Reduced connection overhead by 50%",
						Author: AuthorInfo{
							ID:          "agent_claude",
							Type:        "agent",
							DisplayName: "Claude",
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					{
						ID:        "app-2",
						ProblemID: "prob-app",
						Angle:     "Optimize query structure",
						Status:    "working",
						Author: AuthorInfo{
							ID:          "user-2",
							Type:        "human",
							DisplayName: "Developer",
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "--include", "approaches", "prob-app"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify both endpoints were called
	if len(requestPaths) != 2 {
		t.Errorf("expected 2 API requests, got %d", len(requestPaths))
	}

	output := stdout.String()
	if !strings.Contains(output, "Approaches") {
		t.Error("output should contain Approaches section")
	}
	if !strings.Contains(output, "Try using connection pooling") {
		t.Error("output should contain approach angle")
	}
	if !strings.Contains(output, "succeeded") {
		t.Error("output should contain approach status")
	}
}

func TestE2E_GetCommand_WithAnswers(t *testing.T) {
	requestPaths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)

		if r.URL.Path == "/v1/posts/q-ans" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "q-ans",
					Type:        "question",
					Title:       "Question with answers",
					Description: "Test question",
					Tags:        []string{},
					Status:      "answered",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Test",
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
		} else if r.URL.Path == "/v1/questions/q-ans" {
			response := QuestionAPIResponse{
				Data: QuestionWithAnswers{
					PostDetail: PostDetail{
						ID:          "q-ans",
						Type:        "question",
						Title:       "Question with answers",
						Description: "Test question",
						Tags:        []string{},
						Status:      "answered",
						Author: AuthorInfo{
							ID:          "user-1",
							Type:        "human",
							DisplayName: "Test",
						},
						Upvotes:   5,
						Downvotes: 0,
						VoteScore: 5,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Answers: []AnswerDetail{
						{
							ID:         "ans-1",
							QuestionID: "q-ans",
							Content:    "You can use the time.After function for retries",
							IsAccepted: true,
							Author: AuthorInfo{
								ID:          "agent_claude",
								Type:        "agent",
								DisplayName: "Claude",
							},
							Upvotes:   10,
							Downvotes: 0,
							VoteScore: 10,
							CreatedAt: time.Now(),
						},
						{
							ID:         "ans-2",
							QuestionID: "q-ans",
							Content:    "Consider using a library like backoff",
							IsAccepted: false,
							Author: AuthorInfo{
								ID:          "user-2",
								Type:        "human",
								DisplayName: "Helper",
							},
							Upvotes:   3,
							Downvotes: 1,
							VoteScore: 2,
							CreatedAt: time.Now(),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "--include", "answers", "q-ans"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify both endpoints were called
	if len(requestPaths) != 2 {
		t.Errorf("expected 2 API requests, got %d", len(requestPaths))
	}

	output := stdout.String()
	if !strings.Contains(output, "Answers") {
		t.Error("output should contain Answers section")
	}
	if !strings.Contains(output, "time.After") {
		t.Error("output should contain answer content")
	}
	if !strings.Contains(output, "Accepted") {
		t.Error("output should show accepted answer")
	}
}

func TestE2E_GetCommand_WithResponses(t *testing.T) {
	requestPaths := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)

		if r.URL.Path == "/v1/posts/idea-resp" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "idea-resp",
					Type:        "idea",
					Title:       "Idea with responses",
					Description: "Test idea",
					Tags:        []string{},
					Status:      "exploring",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Test",
					},
					Upvotes:   3,
					Downvotes: 1,
					VoteScore: 2,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/v1/ideas/idea-resp" {
			response := IdeaAPIResponse{
				Data: IdeaWithResponses{
					PostDetail: PostDetail{
						ID:          "idea-resp",
						Type:        "idea",
						Title:       "Idea with responses",
						Description: "Test idea",
						Tags:        []string{},
						Status:      "exploring",
						Author: AuthorInfo{
							ID:          "user-1",
							Type:        "human",
							DisplayName: "Test",
						},
						Upvotes:   3,
						Downvotes: 1,
						VoteScore: 2,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Responses: []ResponseDetail{
						{
							ID:           "resp-1",
							IdeaID:       "idea-resp",
							Content:      "Great idea! We should prototype this.",
							ResponseType: "support",
							Author: AuthorInfo{
								ID:          "agent_gpt4",
								Type:        "agent",
								DisplayName: "GPT-4",
							},
							Upvotes:   5,
							Downvotes: 0,
							VoteScore: 5,
							CreatedAt: time.Now(),
						},
						{
							ID:           "resp-2",
							IdeaID:       "idea-resp",
							Content:      "Have you considered the performance implications?",
							ResponseType: "concern",
							Author: AuthorInfo{
								ID:          "user-2",
								Type:        "human",
								DisplayName: "Reviewer",
							},
							Upvotes:   2,
							Downvotes: 0,
							VoteScore: 2,
							CreatedAt: time.Now(),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "--include", "responses", "idea-resp"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify both endpoints were called
	if len(requestPaths) != 2 {
		t.Errorf("expected 2 API requests, got %d", len(requestPaths))
	}

	output := stdout.String()
	if !strings.Contains(output, "Responses") {
		t.Error("output should contain Responses section")
	}
	if !strings.Contains(output, "Great idea") {
		t.Error("output should contain response content")
	}
	if !strings.Contains(output, "support") {
		t.Error("output should show response type")
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

func TestE2E_GetCommand_JSONWithIncludes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/posts/q-json-inc" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "q-json-inc",
					Type:        "question",
					Title:       "JSON with includes",
					Description: "Test",
					Tags:        []string{},
					Status:      "answered",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Test",
					},
					Upvotes:   1,
					Downvotes: 0,
					VoteScore: 1,
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else if r.URL.Path == "/v1/questions/q-json-inc" {
			response := QuestionAPIResponse{
				Data: QuestionWithAnswers{
					PostDetail: PostDetail{
						ID:          "q-json-inc",
						Type:        "question",
						Title:       "JSON with includes",
						Description: "Test",
						Tags:        []string{},
						Status:      "answered",
						Author: AuthorInfo{
							ID:          "user-1",
							Type:        "human",
							DisplayName: "Test",
						},
						Upvotes:   1,
						Downvotes: 0,
						VoteScore: 1,
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Answers: []AnswerDetail{
						{
							ID:         "ans-json-1",
							QuestionID: "q-json-inc",
							Content:    "JSON answer content",
							IsAccepted: true,
							Author: AuthorInfo{
								ID:          "agent_claude",
								Type:        "agent",
								DisplayName: "Claude",
							},
							Upvotes:   5,
							Downvotes: 0,
							VoteScore: 5,
							CreatedAt: time.Now(),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	}))
	defer server.Close()

	rootCmd := NewRootCmd()
	stdout := new(bytes.Buffer)
	rootCmd.SetOut(stdout)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"get", "--api-url", server.URL + "/v1", "--json", "--include", "answers", "q-json-inc"})

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Verify output is valid JSON with answers included
	var result map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data field in JSON response")
	}

	answers, ok := data["answers"].([]interface{})
	if !ok {
		t.Fatal("expected answers field in data")
	}
	if len(answers) != 1 {
		t.Errorf("expected 1 answer, got %d", len(answers))
	}
}

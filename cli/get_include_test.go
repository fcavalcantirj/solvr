package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestGetCommand_IncludeFlag tests that the --include flag exists
func TestGetCommand_IncludeFlag(t *testing.T) {
	getCmd := NewGetCmd()
	flag := getCmd.Flags().Lookup("include")
	if flag == nil {
		t.Error("get command should have --include flag")
	}
}

// TestGetCommand_IncludeApproaches tests including approaches for a problem
func TestGetCommand_IncludeApproaches(t *testing.T) {
	// Track which endpoints are called
	var postsEndpointCalled bool
	var approachesEndpointCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET /posts/prob-123
		if r.URL.Path == "/posts/prob-123" {
			postsEndpointCalled = true
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "prob-123",
					Type:        "problem",
					Title:       "Race condition problem",
					Description: "We have a race condition when multiple goroutines access the database...",
					Status:      "in_progress",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Developer",
					},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle GET /problems/prob-123/approaches
		if r.URL.Path == "/problems/prob-123/approaches" {
			approachesEndpointCalled = true
			response := ApproachesAPIResponse{
				Data: []ApproachDetail{
					{
						ID:         "approach-1",
						ProblemID:  "prob-123",
						Angle:      "Use mutex locks",
						Method:     "Wrap database access in sync.Mutex",
						Status:     "working",
						AuthorType: "human",
						AuthorID:   "user-2",
						Author: AuthorInfo{
							ID:          "user-2",
							Type:        "human",
							DisplayName: "Helper",
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.Flags().Set("include", "approaches")
	getCmd.SetArgs([]string{"prob-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	if !postsEndpointCalled {
		t.Error("expected posts endpoint to be called")
	}

	if !approachesEndpointCalled {
		t.Error("expected approaches endpoint to be called when --include approaches is set")
	}

	output := buf.String()

	// Check that output contains approaches section
	if !bytes.Contains([]byte(output), []byte("Approaches")) {
		t.Error("output should contain 'Approaches' section header")
	}

	// Check that output contains approach details
	if !bytes.Contains([]byte(output), []byte("Use mutex locks")) {
		t.Error("output should contain the approach angle")
	}
}

// TestGetCommand_IncludeAnswers tests including answers for a question
func TestGetCommand_IncludeAnswers(t *testing.T) {
	// Track which endpoints are called
	var questionsEndpointCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET /posts/q-123 first to get type
		if r.URL.Path == "/posts/q-123" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "q-123",
					Type:        "question",
					Title:       "How do I fix this bug?",
					Description: "I'm encountering a bug when trying to process data asynchronously...",
					Status:      "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Questioner",
					},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle GET /questions/q-123 (includes answers)
		if r.URL.Path == "/questions/q-123" {
			questionsEndpointCalled = true
			response := QuestionAPIResponse{
				Data: QuestionWithAnswers{
					PostDetail: PostDetail{
						ID:          "q-123",
						Type:        "question",
						Title:       "How do I fix this bug?",
						Description: "I'm encountering a bug when trying to process data asynchronously...",
						Status:      "open",
						Author: AuthorInfo{
							ID:          "user-1",
							Type:        "human",
							DisplayName: "Questioner",
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Answers: []AnswerDetail{
						{
							ID:         "answer-1",
							QuestionID: "q-123",
							Content:    "You should try using channels for synchronization.",
							AuthorType: "human",
							AuthorID:   "user-2",
							Author: AuthorInfo{
								ID:          "user-2",
								Type:        "human",
								DisplayName: "Expert",
							},
							IsAccepted: true,
							Upvotes:    10,
							Downvotes:  0,
							VoteScore:  10,
							CreatedAt:  time.Now(),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.Flags().Set("include", "answers")
	getCmd.SetArgs([]string{"q-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	if !questionsEndpointCalled {
		t.Error("expected questions endpoint to be called when --include answers is set")
	}

	output := buf.String()

	// Check that output contains answers section
	if !bytes.Contains([]byte(output), []byte("Answers")) {
		t.Error("output should contain 'Answers' section header")
	}

	// Check that output contains answer details
	if !bytes.Contains([]byte(output), []byte("channels for synchronization")) {
		t.Error("output should contain the answer content")
	}

	// Check that accepted answer is indicated
	if !bytes.Contains([]byte(output), []byte("✓")) || !bytes.Contains([]byte(output), []byte("Accepted")) {
		t.Error("output should indicate the accepted answer")
	}
}

// TestGetCommand_IncludeResponses tests including responses for an idea
func TestGetCommand_IncludeResponses(t *testing.T) {
	// Track which endpoints are called
	var ideasEndpointCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET /posts/idea-123 first to get type
		if r.URL.Path == "/posts/idea-123" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "idea-123",
					Type:        "idea",
					Title:       "New approach to caching",
					Description: "What if we used a distributed cache with automatic invalidation...",
					Status:      "open",
					Author: AuthorInfo{
						ID:          "user-1",
						Type:        "human",
						DisplayName: "Thinker",
					},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		// Handle GET /ideas/idea-123 (includes responses)
		if r.URL.Path == "/ideas/idea-123" {
			ideasEndpointCalled = true
			response := IdeaAPIResponse{
				Data: IdeaWithResponses{
					PostDetail: PostDetail{
						ID:          "idea-123",
						Type:        "idea",
						Title:       "New approach to caching",
						Description: "What if we used a distributed cache with automatic invalidation...",
						Status:      "open",
						Author: AuthorInfo{
							ID:          "user-1",
							Type:        "human",
							DisplayName: "Thinker",
						},
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
					},
					Responses: []ResponseDetail{
						{
							ID:           "response-1",
							IdeaID:       "idea-123",
							Content:      "This is a great idea! We could use Redis for this.",
							ResponseType: "support",
							AuthorType:   "agent",
							AuthorID:     "claude_helper",
							Author: AuthorInfo{
								ID:          "claude_helper",
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
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.Flags().Set("include", "responses")
	getCmd.SetArgs([]string{"idea-123"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("get command failed: %v", err)
	}

	if !ideasEndpointCalled {
		t.Error("expected ideas endpoint to be called when --include responses is set")
	}

	output := buf.String()

	// Check that output contains responses section
	if !bytes.Contains([]byte(output), []byte("Responses")) {
		t.Error("output should contain 'Responses' section header")
	}

	// Check that output contains response details
	if !bytes.Contains([]byte(output), []byte("Redis")) {
		t.Error("output should contain the response content")
	}

	// Check that response type is shown
	if !bytes.Contains([]byte(output), []byte("support")) {
		t.Error("output should show the response type")
	}
}

// TestGetCommand_IncludeMultiple tests including multiple types
func TestGetCommand_IncludeMultiple(t *testing.T) {
	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)

	// Test that we can set multiple include values
	getCmd.Flags().Set("include", "approaches,answers")
	includeVal, _ := getCmd.Flags().GetString("include")
	if includeVal != "approaches,answers" {
		t.Errorf("expected include value 'approaches,answers', got '%s'", includeVal)
	}
}

// TestGetCommand_IncludeHelpText tests that help text mentions --include
func TestGetCommand_IncludeHelpText(t *testing.T) {
	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetArgs([]string{"--help"})

	err := getCmd.Execute()
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}

	output := buf.String()

	// Check that help mentions --include
	if !bytes.Contains([]byte(output), []byte("--include")) {
		t.Error("help should mention '--include' flag")
	}

	// Check that help mentions approaches, answers, responses
	if !bytes.Contains([]byte(output), []byte("approaches")) {
		t.Error("help should mention 'approaches' as an include option")
	}
}

// TestGetCommand_IncludeJSONFormat tests JSON output includes related content
func TestGetCommand_IncludeJSONFormat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle GET /posts/q-123 first
		if r.URL.Path == "/posts/q-123" {
			response := GetAPIResponse{
				Data: PostDetail{
					ID:          "q-123",
					Type:        "question",
					Title:       "Test question",
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
			return
		}

		// Handle GET /questions/q-123
		if r.URL.Path == "/questions/q-123" {
			response := QuestionAPIResponse{
				Data: QuestionWithAnswers{
					PostDetail: PostDetail{
						ID:          "q-123",
						Type:        "question",
						Title:       "Test question",
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
					Answers: []AnswerDetail{
						{
							ID:         "answer-1",
							QuestionID: "q-123",
							Content:    "Test answer",
							Author: AuthorInfo{
								ID:          "user-2",
								Type:        "human",
								DisplayName: "Answerer",
							},
							CreatedAt: time.Now(),
						},
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	getCmd := NewGetCmd()
	buf := new(bytes.Buffer)
	getCmd.SetOut(buf)
	getCmd.SetErr(buf)
	getCmd.Flags().Set("api-url", server.URL)
	getCmd.Flags().Set("include", "answers")
	getCmd.Flags().Set("json", "true")
	getCmd.SetArgs([]string{"q-123"})

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

	// Should contain answers field
	if data, ok := result["data"].(map[string]interface{}); ok {
		if _, hasAnswers := data["answers"]; !hasAnswers {
			t.Error("JSON output should contain 'answers' field when --include answers is set")
		}
	} else {
		t.Error("JSON output should have 'data' object")
	}
}

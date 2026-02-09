package solvr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key")
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got '%s'", client.apiKey)
	}
	if client.baseURL != DefaultBaseURL {
		t.Errorf("expected baseURL '%s', got '%s'", DefaultBaseURL, client.baseURL)
	}
}

func TestClientWithBaseURL(t *testing.T) {
	client := NewClient("test-api-key", WithBaseURL("https://custom.api.com"))
	if client.baseURL != "https://custom.api.com" {
		t.Errorf("expected baseURL 'https://custom.api.com', got '%s'", client.baseURL)
	}
}

func TestClientWithTimeout(t *testing.T) {
	client := NewClient("test-api-key", WithTimeout(5*time.Second))
	if client.httpClient.Timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", client.httpClient.Timeout)
	}
}

func TestSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/search" {
			t.Errorf("expected /v1/search, got %s", r.URL.Path)
		}
		if r.URL.Query().Get("q") != "golang error handling" {
			t.Errorf("expected query 'golang error handling', got '%s'", r.URL.Query().Get("q"))
		}

		// Send response
		resp := SearchResponse{
			Data: []SearchResult{
				{
					ID:          "post-1",
					Type:        "question",
					Title:       "How to handle errors in Go?",
					Description: "Best practices for error handling...",
					Score:       0.95,
				},
			},
			Meta: Meta{
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.Search(context.Background(), "golang error handling", nil)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 result, got %d", len(resp.Data))
	}
	if resp.Data[0].Title != "How to handle errors in Go?" {
		t.Errorf("expected title 'How to handle errors in Go?', got '%s'", resp.Data[0].Title)
	}
}

func TestSearchWithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify query params
		if r.URL.Query().Get("type") != "problem" {
			t.Errorf("expected type 'problem', got '%s'", r.URL.Query().Get("type"))
		}
		if r.URL.Query().Get("limit") != "10" {
			t.Errorf("expected limit '10', got '%s'", r.URL.Query().Get("limit"))
		}

		resp := SearchResponse{Data: []SearchResult{}, Meta: Meta{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	opts := &SearchOptions{
		Type:  "problem",
		Limit: 10,
	}
	_, err := client.Search(context.Background(), "query", opts)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

func TestGetPost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/posts/post-123" {
			t.Errorf("expected /v1/posts/post-123, got %s", r.URL.Path)
		}

		resp := PostResponse{
			Data: Post{
				ID:          "post-123",
				Type:        "question",
				Title:       "Test Question",
				Description: "This is a test question",
				Status:      "open",
				VoteScore:   10,
				Author: Author{
					ID:          "user-1",
					Type:        "human",
					DisplayName: "John Doe",
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.GetPost(context.Background(), "post-123")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}

	if resp.Data.ID != "post-123" {
		t.Errorf("expected ID 'post-123', got '%s'", resp.Data.ID)
	}
	if resp.Data.Title != "Test Question" {
		t.Errorf("expected title 'Test Question', got '%s'", resp.Data.Title)
	}
}

func TestCreatePost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/posts" {
			t.Errorf("expected /v1/posts, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("expected Authorization header 'Bearer test-api-key'")
		}

		var req CreatePostRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Type != "question" {
			t.Errorf("expected type 'question', got '%s'", req.Type)
		}
		if req.Title != "How to test Go code?" {
			t.Errorf("expected title 'How to test Go code?', got '%s'", req.Title)
		}

		w.WriteHeader(http.StatusCreated)
		resp := PostResponse{
			Data: Post{
				ID:          "new-post-1",
				Type:        req.Type,
				Title:       req.Title,
				Description: req.Description,
				Status:      "open",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	req := CreatePostRequest{
		Type:        "question",
		Title:       "How to test Go code?",
		Description: "I want to learn about testing in Go",
		Tags:        []string{"go", "testing"},
	}
	resp, err := client.CreatePost(context.Background(), req)
	if err != nil {
		t.Fatalf("CreatePost failed: %v", err)
	}

	if resp.Data.ID != "new-post-1" {
		t.Errorf("expected ID 'new-post-1', got '%s'", resp.Data.ID)
	}
}

func TestVote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/posts/post-123/vote" {
			t.Errorf("expected /v1/posts/post-123/vote, got %s", r.URL.Path)
		}

		var req VoteRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Direction != "up" {
			t.Errorf("expected direction 'up', got '%s'", req.Direction)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	err := client.Vote(context.Background(), "post-123", VoteUp)
	if err != nil {
		t.Fatalf("Vote failed: %v", err)
	}
}

func TestCreateAnswer(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/questions/q-123/answers" {
			t.Errorf("expected /v1/questions/q-123/answers, got %s", r.URL.Path)
		}

		var req CreateAnswerRequest
		json.NewDecoder(r.Body).Decode(&req)

		w.WriteHeader(http.StatusCreated)
		resp := AnswerResponse{
			Data: Answer{
				ID:      "answer-1",
				Content: req.Content,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	req := CreateAnswerRequest{
		Content: "You should use table-driven tests...",
	}
	resp, err := client.CreateAnswer(context.Background(), "q-123", req)
	if err != nil {
		t.Fatalf("CreateAnswer failed: %v", err)
	}

	if resp.Data.ID != "answer-1" {
		t.Errorf("expected ID 'answer-1', got '%s'", resp.Data.ID)
	}
}

func TestCreateApproach(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/problems/p-123/approaches" {
			t.Errorf("expected /v1/problems/p-123/approaches, got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
		resp := ApproachResponse{
			Data: Approach{
				ID:      "approach-1",
				Content: "My approach is...",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	req := CreateApproachRequest{
		Content: "My approach is...",
	}
	resp, err := client.CreateApproach(context.Background(), "p-123", req)
	if err != nil {
		t.Fatalf("CreateApproach failed: %v", err)
	}

	if resp.Data.ID != "approach-1" {
		t.Errorf("expected ID 'approach-1', got '%s'", resp.Data.ID)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := ErrorResponse{
			Error: APIError{
				Code:    "NOT_FOUND",
				Message: "Post not found",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	_, err := client.GetPost(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "NOT_FOUND" {
		t.Errorf("expected code 'NOT_FOUND', got '%s'", apiErr.Code)
	}
}

func TestContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		json.NewEncoder(w).Encode(SearchResponse{})
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := client.Search(ctx, "test", nil)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

func TestListAgents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/agents" {
			t.Errorf("expected /v1/agents, got %s", r.URL.Path)
		}

		resp := AgentsResponse{
			Data: []Agent{
				{
					ID:          "agent-1",
					DisplayName: "Test Agent",
					Reputation:       100,
					PostCount:   10,
				},
			},
			Meta: Meta{
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.ListAgents(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 agent, got %d", len(resp.Data))
	}
	if resp.Data[0].DisplayName != "Test Agent" {
		t.Errorf("expected display name 'Test Agent', got '%s'", resp.Data[0].DisplayName)
	}
}

func TestListAgentsWithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sort") != "reputation" {
			t.Errorf("expected sort 'reputation', got '%s'", r.URL.Query().Get("sort"))
		}
		if r.URL.Query().Get("status") != "active" {
			t.Errorf("expected status 'active', got '%s'", r.URL.Query().Get("status"))
		}
		if r.URL.Query().Get("per_page") != "10" {
			t.Errorf("expected per_page '10', got '%s'", r.URL.Query().Get("per_page"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("expected page '2', got '%s'", r.URL.Query().Get("page"))
		}

		resp := AgentsResponse{Data: []Agent{}, Meta: Meta{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	opts := &ListAgentsOptions{
		Sort:   "reputation",
		Status: "active",
		Limit:  10,
		Offset: 20, // offset 20 with limit 10 = page 2
	}
	_, err := client.ListAgents(context.Background(), opts)
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
}

func TestListPosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/posts" {
			t.Errorf("expected /v1/posts, got %s", r.URL.Path)
		}

		resp := PostsResponse{
			Data: []Post{
				{
					ID:    "post-1",
					Type:  "question",
					Title: "Test Post",
				},
			},
			Meta: Meta{
				Total:   1,
				Page:    1,
				PerPage: 20,
				HasMore: false,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	resp, err := client.ListPosts(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListPosts failed: %v", err)
	}

	if len(resp.Data) != 1 {
		t.Errorf("expected 1 post, got %d", len(resp.Data))
	}
}

func TestListPostsWithOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("type") != "problem" {
			t.Errorf("expected type 'problem', got '%s'", r.URL.Query().Get("type"))
		}
		if r.URL.Query().Get("status") != "open" {
			t.Errorf("expected status 'open', got '%s'", r.URL.Query().Get("status"))
		}
		if r.URL.Query().Get("per_page") != "10" {
			t.Errorf("expected per_page '10', got '%s'", r.URL.Query().Get("per_page"))
		}

		resp := PostsResponse{Data: []Post{}, Meta: Meta{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	opts := &SearchOptions{
		Type:   "problem",
		Status: "open",
		Limit:  10,
		Offset: 20,
	}
	_, err := client.ListPosts(context.Background(), opts)
	if err != nil {
		t.Fatalf("ListPosts failed: %v", err)
	}
}

func TestGetAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/v1/agents/agent-123" {
			t.Errorf("expected /v1/agents/agent-123, got %s", r.URL.Path)
		}

		resp := struct {
			Data Agent `json:"data"`
		}{
			Data: Agent{
				ID:          "agent-123",
				DisplayName: "Claude Agent",
				Reputation:       500,
				PostCount:   25,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	agent, err := client.GetAgent(context.Background(), "agent-123")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}

	if agent.ID != "agent-123" {
		t.Errorf("expected ID 'agent-123', got '%s'", agent.ID)
	}
	if agent.DisplayName != "Claude Agent" {
		t.Errorf("expected display name 'Claude Agent', got '%s'", agent.DisplayName)
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 60 * time.Second,
	}
	client := NewClient("test-api-key", WithHTTPClient(customClient))
	if client.httpClient != customClient {
		t.Error("expected custom HTTP client to be set")
	}
}

func TestWithMaxRetries(t *testing.T) {
	client := NewClient("test-api-key", WithMaxRetries(5))
	if client.maxRetries != 5 {
		t.Errorf("expected maxRetries 5, got %d", client.maxRetries)
	}
}

func TestRetryOnNetworkError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			// Simulate network error by closing connection
			hj, ok := w.(http.Hijacker)
			if ok {
				conn, _, _ := hj.Hijack()
				conn.Close()
				return
			}
		}
		// Third attempt succeeds
		json.NewEncoder(w).Encode(SearchResponse{Data: []SearchResult{}, Meta: Meta{}})
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL), WithMaxRetries(3))
	_, err := client.Search(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("Search should have succeeded after retries: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestSearchWithAllOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "open" {
			t.Errorf("expected status 'open', got '%s'", r.URL.Query().Get("status"))
		}
		if r.URL.Query().Get("offset") != "10" {
			t.Errorf("expected offset '10', got '%s'", r.URL.Query().Get("offset"))
		}
		tags := r.URL.Query()["tags"]
		if len(tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(tags))
		}

		resp := SearchResponse{Data: []SearchResult{}, Meta: Meta{}}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	opts := &SearchOptions{
		Status: "open",
		Offset: 10,
		Tags:   []string{"go", "testing"},
	}
	_, err := client.Search(context.Background(), "query", opts)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

func TestHTTPErrorWithoutJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := NewClient("test-api-key", WithBaseURL(server.URL))
	_, err := client.GetPost(context.Background(), "post-1")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != "HTTP_500" {
		t.Errorf("expected code 'HTTP_500', got '%s'", apiErr.Code)
	}
}

func TestAgentReputationDeserialization(t *testing.T) {
	// The API returns "reputation" (not "reputation") since the rename.
	// This test verifies the SDK Agent struct deserializes "reputation" correctly.
	jsonData := `{"id":"agent-1","display_name":"Test","status":"active","reputation":150,"post_count":5}`

	var agent Agent
	if err := json.Unmarshal([]byte(jsonData), &agent); err != nil {
		t.Fatalf("failed to unmarshal agent: %v", err)
	}

	if agent.Reputation != 150 {
		t.Errorf("expected Reputation 150, got %d", agent.Reputation)
	}
}

func TestAPIErrorString(t *testing.T) {
	err := &APIError{
		Code:    "NOT_FOUND",
		Message: "Resource not found",
	}
	expected := "NOT_FOUND: Resource not found"
	if err.Error() != expected {
		t.Errorf("expected '%s', got '%s'", expected, err.Error())
	}
}

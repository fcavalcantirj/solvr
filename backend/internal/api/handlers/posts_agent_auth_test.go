// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// FIX-003: Agent API Key Authentication Tests for Posts Routes
// Per SPEC.md: Both humans (JWT) and AI agents (API key) can post content.
// The posts handlers should accept both authentication methods.
// ============================================================================

// Helper to add agent auth context to request (simulates API key authentication)
func addAgentContext(r *http.Request, agentID string) *http.Request {
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	ctx := auth.ContextWithAgent(r.Context(), agent)
	return r.WithContext(ctx)
}

// TestCreatePost_AgentAuth tests that agents with API key auth can create posts.
func TestCreatePost_AgentAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "Test Problem Title Posted By Agent",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation requirements.",
		"tags":        []string{"go", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentContext(req, "test-agent-123")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data := resp["data"].(map[string]interface{})
	if data["id"] == nil {
		t.Error("expected post id in response")
	}

	// Verify the post was created with agent author type
	if repo.createdPost == nil {
		t.Fatal("expected post to be created")
	}

	if repo.createdPost.PostedByType != models.AuthorTypeAgent {
		t.Errorf("expected posted_by_type 'clawd', got '%s'", repo.createdPost.PostedByType)
	}

	if repo.createdPost.PostedByID != "test-agent-123" {
		t.Errorf("expected posted_by_id 'test-agent-123', got '%s'", repo.createdPost.PostedByID)
	}
}

// TestCreatePost_AgentAuth_Question tests agents can create questions.
func TestCreatePost_AgentAuth_Question(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "question",
		"title":       "How do I handle async operations in Go?",
		"description": "I need to understand how to properly handle async operations in Go. What are the best practices for concurrency?",
		"tags":        []string{"go", "async", "concurrency"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentContext(req, "claude-agent")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	if repo.createdPost.Type != models.PostTypeQuestion {
		t.Errorf("expected type 'question', got '%s'", repo.createdPost.Type)
	}
}

// TestCreatePost_AgentAuth_Idea tests agents can create ideas.
func TestCreatePost_AgentAuth_Idea(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "idea",
		"title":       "Observation about async patterns in modern codebases",
		"description": "I've noticed that most async bugs stem from improper error handling in concurrent code. Here are my observations and suggestions for improvement.",
		"tags":        []string{"patterns", "async", "best-practices"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAgentContext(req, "gpt-agent")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	if repo.createdPost.Type != models.PostTypeIdea {
		t.Errorf("expected type 'idea', got '%s'", repo.createdPost.Type)
	}
}

// TestVotePost_AgentAuth tests that agents can vote on posts.
func TestVotePost_AgentAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	// Create a post by a different user (so agent can vote on it)
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByType = models.AuthorTypeHuman
	post.PostedByID = "human-user-456"
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAgentContext(req, "voting-agent")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify vote was recorded with agent voter type
	if repo.vote == nil {
		t.Fatal("expected vote to be recorded")
	}

	if repo.vote.VoterType != string(models.AuthorTypeAgent) {
		t.Errorf("expected voter_type 'clawd', got '%s'", repo.vote.VoterType)
	}

	if repo.vote.VoterID != "voting-agent" {
		t.Errorf("expected voter_id 'voting-agent', got '%s'", repo.vote.VoterID)
	}
}

// TestVotePost_AgentCannotVoteOwnContent tests agents cannot vote on their own content.
func TestVotePost_AgentCannotVoteOwnContent(t *testing.T) {
	repo := NewMockPostsRepository()
	// Create a post by the same agent
	now := time.Now()
	post := models.PostWithAuthor{
		Post: models.Post{
			ID:           "agent-post-123",
			Type:         models.PostTypeProblem,
			Title:        "Agent's Own Post",
			Description:  "This is a post created by an agent, description must be at least fifty characters long.",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   "self-voting-agent",
			Status:       models.PostStatusOpen,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeAgent,
			ID:          "self-voting-agent",
			DisplayName: "Self Voting Agent",
		},
	}
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "up",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/agent-post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "agent-post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAgentContext(req, "self-voting-agent") // Same agent as post author
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestUpdatePost_AgentCanUpdateOwnPost tests agents can update their own posts.
func TestUpdatePost_AgentCanUpdateOwnPost(t *testing.T) {
	repo := NewMockPostsRepository()
	now := time.Now()
	post := models.PostWithAuthor{
		Post: models.Post{
			ID:           "agent-post-456",
			Type:         models.PostTypeProblem,
			Title:        "Original Title Needs To Be Long Enough",
			Description:  "Original description that is at least fifty characters long for validation purposes.",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   "updating-agent",
			Status:       models.PostStatusOpen,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeAgent,
			ID:          "updating-agent",
			DisplayName: "Updating Agent",
		},
	}
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	newTitle := "Updated Title That Is Also Long Enough"
	body := map[string]interface{}{
		"title": newTitle,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/agent-post-456", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "agent-post-456")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAgentContext(req, "updating-agent") // Same agent as post author
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}

	if repo.updatedPost.Title != newTitle {
		t.Errorf("expected title '%s', got '%s'", newTitle, repo.updatedPost.Title)
	}
}

// TestUpdatePost_AgentCannotUpdateOthersPost tests agents cannot update others' posts.
func TestUpdatePost_AgentCannotUpdateOthersPost(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("human-post-789", "Human's Post", models.PostTypeProblem)
	// Post is owned by human user-123
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Attempted Update By Agent",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/human-post-789", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "human-post-789")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAgentContext(req, "unauthorized-agent")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestDeletePost_AgentCanDeleteOwnPost tests agents can delete their own posts.
func TestDeletePost_AgentCanDeleteOwnPost(t *testing.T) {
	repo := NewMockPostsRepository()
	now := time.Now()
	post := models.PostWithAuthor{
		Post: models.Post{
			ID:           "agent-post-delete",
			Type:         models.PostTypeProblem,
			Title:        "Agent's Post To Delete",
			Description:  "Description that is at least fifty characters long for validation purposes.",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   "deleting-agent",
			Status:       models.PostStatusOpen,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeAgent,
			ID:          "deleting-agent",
			DisplayName: "Deleting Agent",
		},
	}
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/agent-post-delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "agent-post-delete")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAgentContext(req, "deleting-agent")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d. Body: %s", w.Code, w.Body.String())
	}

	if repo.deletedID != "agent-post-delete" {
		t.Errorf("expected deletedID 'agent-post-delete', got '%s'", repo.deletedID)
	}
}

// TestDeletePost_AgentCannotDeleteOthersPost tests agents cannot delete others' posts.
func TestDeletePost_AgentCannotDeleteOthersPost(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("human-post-delete", "Human's Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/human-post-delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "human-post-delete")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAgentContext(req, "unauthorized-deleting-agent")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestBothAuthMethods_JWTStillWorks tests that JWT authentication still works after changes.
func TestBothAuthMethods_JWTStillWorks(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"type":        "problem",
		"title":       "JWT Auth Test Problem Title Here",
		"description": "This is a test description that needs to be at least fifty characters long to pass validation.",
		"tags":        []string{"jwt", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req = addAuthContext(req, "jwt-user-123", "user") // JWT auth
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d. Body: %s", w.Code, w.Body.String())
	}

	if repo.createdPost.PostedByType != models.AuthorTypeHuman {
		t.Errorf("expected posted_by_type 'human', got '%s'", repo.createdPost.PostedByType)
	}

	if repo.createdPost.PostedByID != "jwt-user-123" {
		t.Errorf("expected posted_by_id 'jwt-user-123', got '%s'", repo.createdPost.PostedByID)
	}
}

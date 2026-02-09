package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// POST /v1/posts/:id/vote - Vote Tests
// ============================================================================

// TestVote_Upvote tests successful upvote.
func TestVote_Upvote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user" // Different from voter
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
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.vote.Direction != "up" {
		t.Errorf("expected direction 'up', got '%s'", repo.vote.Direction)
	}
}

// TestVote_ResponseIncludesScores tests vote response includes data.vote_score, upvotes, downvotes.
func TestVote_ResponseIncludesScores(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user"
	post.Upvotes = 10
	post.Downvotes = 2
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
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	if _, ok := data["vote_score"]; !ok {
		t.Error("expected vote_score in response data")
	}
	if _, ok := data["upvotes"]; !ok {
		t.Error("expected upvotes in response data")
	}
	if _, ok := data["downvotes"]; !ok {
		t.Error("expected downvotes in response data")
	}
	if data["user_vote"] != "up" {
		t.Errorf("expected user_vote 'up', got %v", data["user_vote"])
	}
}

// TestVote_Downvote tests successful downvote.
func TestVote_Downvote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user"
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "down",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.vote.Direction != "down" {
		t.Errorf("expected direction 'down', got '%s'", repo.vote.Direction)
	}
}

// TestVote_InvalidDirection tests 400 for invalid direction.
func TestVote_InvalidDirection(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"direction": "sideways",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts/post-123/vote", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestVote_NoAuth tests 401 when not authenticated.
func TestVote_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
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
	// No auth context
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestVote_DuplicateVote tests 409 for duplicate vote.
func TestVote_DuplicateVote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	post.PostedByID = "other-user"
	repo.SetPost(&post)
	repo.SetVoteError(ErrDuplicateVote)

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
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Vote(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}
}

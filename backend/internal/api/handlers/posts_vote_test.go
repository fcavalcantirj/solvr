package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
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

// ============================================================================
// GET /v1/posts/:id/my-vote - GetMyVote Tests
// ============================================================================

// TestGetMyVote_Success tests successful fetch of user's vote.
func TestGetMyVote_Success_Upvote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)
	upvote := "up"
	repo.SetUserVote(&upvote)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123/my-vote", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.GetMyVote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	vote, ok := data["vote"].(string)
	if !ok {
		t.Fatal("expected vote string in response data")
	}

	if vote != "up" {
		t.Errorf("expected vote 'up', got %s", vote)
	}
}

// TestGetMyVote_NoVote tests when user hasn't voted.
func TestGetMyVote_NoVote(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)
	repo.SetUserVote(nil) // No vote

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123/my-vote", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.GetMyVote(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	vote := data["vote"]
	if vote != nil {
		t.Errorf("expected vote nil, got %v", vote)
	}
}

// TestGetMyVote_Unauthorized tests 401 when not authenticated.
func TestGetMyVote_Unauthorized(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123/my-vote", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.GetMyVote(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestPostsHandler_List_UserVoteNullExplicit tests that GET /v1/posts for an
// authenticated user who has NOT voted includes "user_vote": null explicitly
// in the JSON response (field present with null value, NOT omitted).
// This prevents N+1 /my-vote calls on the frontend.
func TestPostsHandler_List_UserVoteNullExplicit(t *testing.T) {
	repo := NewMockPostsRepository()

	// Post with UserVote = nil (user hasn't voted)
	post := createTestPost("post-xyz", "Some Post", models.PostTypeProblem)
	post.PostedByID = "other-user-456"
	post.UserVote = nil // Explicitly no vote
	repo.SetPosts([]models.PostWithAuthor{post}, 1)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	req = addAuthContext(req, "viewer-user-123", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Parse response
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 post, got %d", len(data))
	}

	postData, ok := data[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected post object in data array")
	}

	// The "user_vote" key MUST be present in the JSON (even if null).
	// If the field is omitempty and nil, it will be absent — that's the bug.
	if _, exists := postData["user_vote"]; !exists {
		t.Error("expected 'user_vote' key to be present in post JSON (even when null) to prevent N+1 frontend fetches; field was absent (omitempty bug)")
	}

	// And its value must be null (not some stale vote value)
	if postData["user_vote"] != nil {
		t.Errorf("expected 'user_vote' to be null, got %v", postData["user_vote"])
	}
}

// TestGetMyVote_PostNotFound tests 404 when post doesn't exist.
func TestGetMyVote_PostNotFound(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetError(db.ErrPostNotFound)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/non-existent/my-vote", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "non-existent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.GetMyVote(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

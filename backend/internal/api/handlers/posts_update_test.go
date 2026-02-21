package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// PATCH /v1/posts/:id - Update Post Tests
// ============================================================================

// TestUpdatePost_Success tests successful post update.
func TestUpdatePost_Success(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title That Is Long Enough",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user") // Same as post owner
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	if repo.updatedPost == nil {
		t.Error("expected post to be updated")
	}
}

// TestUpdatePost_NotOwner tests 403 for non-owner.
func TestUpdatePost_NotOwner(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "other-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("expected error code FORBIDDEN, got %v", errObj["code"])
	}
}

// TestUpdatePost_NoAuth tests 401 when not authenticated.
func TestUpdatePost_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestUpdatePost_NotFound tests 404 when post doesn't exist.
func TestUpdatePost_NotFound(t *testing.T) {
	repo := NewMockPostsRepository()
	repo.SetPost(nil)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestUpdatePost_TooManyTags tests 400 for more than 10 tags on update.
func TestUpdatePost_TooManyTags(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title", models.PostTypeProblem)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	tags := make([]string, 11)
	for i := range tags {
		tags[i] = "tag"
	}

	body := map[string]interface{}{
		"tags": tags,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
	}
	if errObj["message"] != "maximum 10 tags allowed" {
		t.Errorf("expected error message 'maximum 10 tags allowed', got %v", errObj["message"])
	}
}

// ============================================================================
// Status Guard & Re-Moderation Tests
// ============================================================================

// createTestPostWithStatus creates a test post with a specific status.
func createTestPostWithStatus(id, title string, postType models.PostType, status models.PostStatus) models.PostWithAuthor {
	now := time.Now()
	return models.PostWithAuthor{
		Post: models.Post{
			ID:           id,
			Type:         postType,
			Title:        title,
			Description:  "Test description with more than fifty characters for validation purposes here",
			Tags:         []string{"test", "go"},
			PostedByType: models.AuthorTypeHuman,
			PostedByID:   "user-123",
			Status:       status,
			Upvotes:      10,
			Downvotes:    2,
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		Author: models.PostAuthor{
			Type:        models.AuthorTypeHuman,
			ID:          "user-123",
			DisplayName: "Test User",
			AvatarURL:   "https://example.com/avatar.png",
		},
		VoteScore: 8,
	}
}

// TestUpdatePost_CannotEditSolved tests that editing a solved post returns 400.
func TestUpdatePost_CannotEditSolved(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-123", "Solved Problem Post", models.PostTypeProblem, models.PostStatusSolved)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title That Is Long Enough",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	errObj := resp["error"].(map[string]interface{})
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "solved") {
		t.Errorf("expected error message to mention 'solved', got %q", msg)
	}
}

// TestUpdatePost_CannotEditAnswered tests that editing an answered question returns 400.
func TestUpdatePost_CannotEditAnswered(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-123", "Answered Question Post", models.PostTypeQuestion, models.PostStatusAnswered)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	body := map[string]interface{}{
		"title": "Updated Title That Is Long Enough",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	errObj := resp["error"].(map[string]interface{})
	msg := errObj["message"].(string)
	if !strings.Contains(msg, "answered") {
		t.Errorf("expected error message to mention 'answered', got %q", msg)
	}
}

// TestUpdatePost_RejectedTriggersReModeration tests that editing a rejected post
// changes status to pending_review and triggers re-moderation.
func TestUpdatePost_RejectedTriggersReModeration(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-123", "Rejected Post Title Here", models.PostTypeProblem, models.PostStatusRejected)
	repo.SetPost(&post)

	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{Approved: true, Explanation: "OK"})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	body := map[string]interface{}{
		"title": "Updated Title That Is Long Enough For Validation",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the post was saved with pending_review status
	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}
	if repo.updatedPost.Status != models.PostStatusPendingReview {
		t.Errorf("expected status %q, got %q", models.PostStatusPendingReview, repo.updatedPost.Status)
	}

	// Wait for async moderation goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Verify moderation service was called
	if calls := modService.GetCalls(); calls != 1 {
		t.Errorf("expected 1 moderation call, got %d", calls)
	}
}

// TestUpdatePost_OpenContentChangeTriggersReModeration tests that editing content
// of an open post triggers re-moderation (status becomes pending_review).
func TestUpdatePost_OpenContentChangeTriggersReModeration(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-123", "Original Open Post Title", models.PostTypeProblem, models.PostStatusOpen)
	repo.SetPost(&post)

	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{Approved: true, Explanation: "OK"})

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	body := map[string]interface{}{
		"title": "Changed Title For Open Post Needs Moderation",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the post was saved with pending_review status
	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}
	if repo.updatedPost.Status != models.PostStatusPendingReview {
		t.Errorf("expected status %q, got %q", models.PostStatusPendingReview, repo.updatedPost.Status)
	}

	// Wait for async moderation goroutine to complete
	time.Sleep(200 * time.Millisecond)

	// Verify moderation service was called
	if calls := modService.GetCalls(); calls != 1 {
		t.Errorf("expected 1 moderation call, got %d", calls)
	}
}

// TestUpdatePost_OpenTagsOnlyNoReModeration tests that editing only tags
// on an open post does NOT trigger re-moderation (tags-only change stays open).
func TestUpdatePost_OpenTagsOnlyNoReModeration(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-123", "Original Open Post Title", models.PostTypeProblem, models.PostStatusOpen)
	repo.SetPost(&post)

	statusUpdater := NewMockPostStatusUpdater()
	modService := NewMockContentModerationService()

	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(statusUpdater)

	body := map[string]interface{}{
		"tags": []string{"golang", "testing"},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the post was saved with status still open (not pending_review)
	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}
	if repo.updatedPost.Status != models.PostStatusOpen {
		t.Errorf("expected status to remain %q, got %q", models.PostStatusOpen, repo.updatedPost.Status)
	}

	// Give some time to ensure no goroutine was spawned
	time.Sleep(100 * time.Millisecond)

	// Verify moderation service was NOT called
	if calls := modService.GetCalls(); calls != 0 {
		t.Errorf("expected 0 moderation calls for tags-only update, got %d", calls)
	}
}

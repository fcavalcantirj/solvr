package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// DELETE /v1/posts/:id - Delete Post Tests
// ============================================================================

// TestDeletePost_OwnerCanDelete tests owner can delete their post.
func TestDeletePost_OwnerCanDelete(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypePost)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user") // Same as post owner
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	if repo.deletedID != "post-123" {
		t.Errorf("expected deletedID 'post-123', got '%s'", repo.deletedID)
	}
}

// TestDeletePost_AdminCanDelete tests admin can delete any post.
func TestDeletePost_AdminCanDelete(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypePost)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "admin-user", "admin") // Admin, different from owner
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

// TestDeletePost_OthersForbidden tests non-owner non-admin gets 403.
func TestDeletePost_OthersForbidden(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Test Post", models.PostTypePost)
	repo.SetPost(&post)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "other-user", "user") // Different user, not admin
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestDeletePost_NoAuth tests 401 when not authenticated.
func TestDeletePost_NoAuth(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

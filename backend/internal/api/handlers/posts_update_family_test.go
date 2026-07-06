package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// TestUpdatePost_FamilyContentChange_NoReModeration is the BART-154 Update guard: editing a
// FAMILY post's content keeps status=open and never fires moderation (family posts are never
// re-moderated). RED before the guard is added.
func TestUpdatePost_FamilyContentChange_NoReModeration(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-fam", "Family Post Title Here", models.PostTypeProblem, models.PostStatusOpen)
	post.Visibility = models.VisibilityFamily // the field FindByIDForViewer populates in prod
	repo.SetPost(&post)

	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{Approved: true, Explanation: "OK"})
	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(NewMockPostStatusUpdater())
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond})

	body, _ := json.Marshal(map[string]any{"title": "Edited Family Title That Is Long Enough"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-fam", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-fam")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.updatedPost == nil {
		t.Fatal("expected post to be updated")
	}
	if repo.updatedPost.Status != models.PostStatusOpen {
		t.Errorf("family post status must stay open, got %q", repo.updatedPost.Status)
	}
	time.Sleep(100 * time.Millisecond)
	if calls := modService.GetCalls(); calls != 0 {
		t.Errorf("expected 0 moderation calls for family post, got %d", calls)
	}
}

// TestUpdatePost_PublicContentChange_ReModerates is the control: a PUBLIC post still flips to
// pending_review and calls moderation once (unchanged behavior).
func TestUpdatePost_PublicContentChange_ReModerates(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPostWithStatus("post-pub", "Public Post Title Here", models.PostTypeProblem, models.PostStatusOpen)
	post.Visibility = models.VisibilityPublic
	repo.SetPost(&post)

	modService := NewMockContentModerationService()
	modService.SetResult(&ModerationResult{Approved: true, Explanation: "OK"})
	handler := NewPostsHandler(repo)
	handler.SetContentModerationService(modService)
	handler.SetPostStatusUpdater(NewMockPostStatusUpdater())
	handler.SetRetryDelays([]time.Duration{10 * time.Millisecond})

	body, _ := json.Marshal(map[string]any{"title": "Edited Public Title That Is Long Enough"})
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-pub", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-pub")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if repo.updatedPost.Status != models.PostStatusPendingReview {
		t.Errorf("public post must flip to pending_review, got %q", repo.updatedPost.Status)
	}
	time.Sleep(200 * time.Millisecond)
	if calls := modService.GetCalls(); calls != 1 {
		t.Errorf("expected 1 moderation call for public post, got %d", calls)
	}
}

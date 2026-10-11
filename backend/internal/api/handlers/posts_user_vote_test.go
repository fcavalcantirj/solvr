package handlers

import (
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
// user_vote viewer info tests
// ============================================================================

// TestListPosts_IncludesUserVote_Authenticated tests that authenticated requests
// pass viewer info to the repo and user_vote is included in response.
func TestListPosts_IncludesUserVote_Authenticated(t *testing.T) {
	repo := NewMockPostsRepository()
	upVote := "up"
	post := createTestPost("post-1", "Test Post", models.PostTypePost)
	post.UserVote = &upVote
	repo.SetPosts([]models.PostWithAuthor{post}, 1)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	req = addAuthContext(req, "viewer-user-1", "user")
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify repo received viewer info in list options
	if repo.listOpts.ViewerType != models.AuthorTypeHuman {
		t.Errorf("expected ViewerType 'human', got '%s'", repo.listOpts.ViewerType)
	}
	if repo.listOpts.ViewerID != "viewer-user-1" {
		t.Errorf("expected ViewerID 'viewer-user-1', got '%s'", repo.listOpts.ViewerID)
	}

	// Verify user_vote appears in response JSON
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := resp["data"].([]interface{})
	first := data[0].(map[string]interface{})
	userVote, ok := first["user_vote"]
	if !ok {
		t.Fatal("expected user_vote field in response when authenticated")
	}
	if userVote != "up" {
		t.Errorf("expected user_vote 'up', got '%v'", userVote)
	}
}

// TestListPosts_NoUserVote_Anonymous tests that anonymous requests
// do NOT pass viewer info and user_vote is omitted from response.
func TestListPosts_NoUserVote_Anonymous(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-1", "Test Post", models.PostTypePost)
	// UserVote is nil (anonymous)
	repo.SetPosts([]models.PostWithAuthor{post}, 1)

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
	// No auth context
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify repo did NOT receive viewer info
	if repo.listOpts.ViewerType != "" {
		t.Errorf("expected empty ViewerType for anonymous, got '%s'", repo.listOpts.ViewerType)
	}
	if repo.listOpts.ViewerID != "" {
		t.Errorf("expected empty ViewerID for anonymous, got '%s'", repo.listOpts.ViewerID)
	}

	// Verify user_vote is present and null (anonymous = no vote, field always serialized)
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := resp["data"].([]interface{})
	first := data[0].(map[string]interface{})
	if _, ok := first["user_vote"]; !ok {
		t.Error("expected user_vote key to be present (as null) for anonymous request")
	}
	if first["user_vote"] != nil {
		t.Errorf("expected user_vote to be null for anonymous request, got %v", first["user_vote"])
	}
}

// TestGetPost_IncludesUserVote_Authenticated tests that authenticated GET /v1/posts/:id
// uses FindByIDForViewer and includes user_vote in response.
func TestGetPost_IncludesUserVote_Authenticated(t *testing.T) {
	repo := &MockPostsRepoWithViewerTracking{}
	downVote := "down"
	post := createTestPost("post-123", "Test Post", models.PostTypePost)
	post.UserVote = &downVote
	repo.post = &post

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "viewer-user-2", "user")
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify FindByIDForViewer was called (not FindByID)
	if !repo.findByIDForViewerCalled {
		t.Error("expected FindByIDForViewer to be called for authenticated request")
	}
	if repo.calledViewerType != models.AuthorTypeHuman {
		t.Errorf("expected viewer type 'human', got '%s'", repo.calledViewerType)
	}
	if repo.calledViewerID != "viewer-user-2" {
		t.Errorf("expected viewer ID 'viewer-user-2', got '%s'", repo.calledViewerID)
	}

	// Verify user_vote in response
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	userVote, ok := data["user_vote"]
	if !ok {
		t.Fatal("expected user_vote field in response for authenticated request")
	}
	if userVote != "down" {
		t.Errorf("expected user_vote 'down', got '%v'", userVote)
	}
}

// TestGetPost_Anonymous_UsesFindByID tests that anonymous GET /v1/posts/:id
// uses FindByID (not FindByIDForViewer) and omits user_vote.
func TestGetPost_Anonymous_UsesFindByID(t *testing.T) {
	repo := &MockPostsRepoWithViewerTracking{}
	post := createTestPost("post-123", "Test Post", models.PostTypePost)
	repo.post = &post

	handler := NewPostsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/posts/post-123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify FindByID was called (not FindByIDForViewer)
	if repo.findByIDForViewerCalled {
		t.Error("expected FindByID to be called for anonymous request, but FindByIDForViewer was called")
	}
	if !repo.findByIDCalled {
		t.Error("expected FindByID to be called for anonymous request")
	}
}

// MockPostsRepoWithViewerTracking tracks which Find method is called.
type MockPostsRepoWithViewerTracking struct {
	MockPostsRepository
	findByIDCalled          bool
	findByIDForViewerCalled bool
	calledViewerType        models.AuthorType
	calledViewerID          string
}

func (m *MockPostsRepoWithViewerTracking) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	m.findByIDCalled = true
	if m.post == nil {
		return nil, db.ErrPostNotFound
	}
	return m.post, nil
}

func (m *MockPostsRepoWithViewerTracking) FindByIDForViewer(ctx context.Context, id string, viewerType models.AuthorType, viewerID string, callerHuman string) (*models.PostWithAuthor, error) {
	m.findByIDForViewerCalled = true
	m.calledViewerType = viewerType
	m.calledViewerID = viewerID
	if m.post == nil {
		return nil, db.ErrPostNotFound
	}
	return m.post, nil
}

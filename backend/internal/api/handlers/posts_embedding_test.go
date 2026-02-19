package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// ============================================================================
// MockEmbeddingService for testing embedding generation
// ============================================================================

type MockEmbeddingService struct {
	embedding []float32
	err       error
	callCount int
	lastInput string
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	m.callCount++
	m.lastInput = text
	if m.err != nil {
		return nil, m.err
	}
	return m.embedding, nil
}

func (m *MockEmbeddingService) GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error) {
	return m.GenerateEmbedding(ctx, text)
}

// Helper to create a valid post JSON body
func validPostBody() string {
	return `{"type":"question","title":"How to handle async operations in Go","description":"I am looking for a detailed explanation on how to handle asynchronous operations in Go using goroutines and channels effectively."}`
}

// ============================================================================
// POST /v1/posts - Create Post with Embedding Tests
// ============================================================================

// TestCreatePost_WithEmbeddingService tests that embedding is generated on create.
func TestCreatePost_WithEmbeddingService(t *testing.T) {
	repo := NewMockPostsRepository()
	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.1, 0.2, 0.3},
	}
	handler := NewPostsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(validPostBody()))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}

	// Verify embedding text combines title + description
	if !strings.Contains(mockEmbed.lastInput, "How to handle async operations in Go") {
		t.Errorf("expected title in embedding input, got: %s", mockEmbed.lastInput)
	}

	// Verify the post has embedding set
	if repo.createdPost == nil {
		t.Fatal("expected created post to be set in repo")
	}
	if repo.createdPost.EmbeddingStr == nil {
		t.Error("expected embedding string to be set on created post")
	}
}

// TestCreatePost_EmbeddingServiceFailure tests graceful degradation on embedding error.
func TestCreatePost_EmbeddingServiceFailure(t *testing.T) {
	repo := NewMockPostsRepository()
	mockEmbed := &MockEmbeddingService{
		err: fmt.Errorf("embedding API unavailable"),
	}
	handler := NewPostsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(validPostBody()))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	// Post should still be created despite embedding failure
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 despite embedding failure, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}

	// Embedding should be nil on the post
	if repo.createdPost.EmbeddingStr != nil {
		t.Error("expected nil embedding on failure")
	}
}

// TestCreatePost_NoEmbeddingService tests normal post creation without embedding service.
func TestCreatePost_NoEmbeddingService(t *testing.T) {
	repo := NewMockPostsRepository()
	handler := NewPostsHandler(repo) // No embedding service set

	req := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(validPostBody()))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdPost.EmbeddingStr != nil {
		t.Error("expected nil embedding when no service set")
	}
}

// ============================================================================
// PATCH /v1/posts/:id - Update Post with Embedding Tests
// ============================================================================

// TestUpdatePost_TitleChangeRegeneratesEmbedding tests embedding regeneration on title change.
func TestUpdatePost_TitleChangeRegeneratesEmbedding(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Original Title for Testing", models.PostTypeQuestion)
	repo.SetPost(&post)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.4, 0.5, 0.6},
	}
	handler := NewPostsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := `{"title":"Updated Title for Semantic Search"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding regenerated on title change, called %d times", mockEmbed.callCount)
	}

	if repo.updatedPost == nil || repo.updatedPost.EmbeddingStr == nil {
		t.Error("expected embedding set on updated post")
	}
}

// TestUpdatePost_DescriptionChangeRegeneratesEmbedding tests embedding regeneration on description change.
func TestUpdatePost_DescriptionChangeRegeneratesEmbedding(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Existing Title for Testing", models.PostTypeQuestion)
	repo.SetPost(&post)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.7, 0.8, 0.9},
	}
	handler := NewPostsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := `{"description":"This is a completely new and updated description that should trigger embedding regeneration for semantic search."}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding regenerated on description change, called %d times", mockEmbed.callCount)
	}
}

// TestUpdatePost_StatusOnlyNoEmbeddingRegeneration tests no embedding when only status changes.
func TestUpdatePost_StatusOnlyNoEmbeddingRegeneration(t *testing.T) {
	repo := NewMockPostsRepository()
	post := createTestPost("post-123", "Existing Title for Testing", models.PostTypeQuestion)
	repo.SetPost(&post)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.1, 0.2, 0.3},
	}
	handler := NewPostsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := `{"status":"answered"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/posts/post-123", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "post-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 0 {
		t.Errorf("expected no embedding regeneration for status-only change, called %d times", mockEmbed.callCount)
	}
}

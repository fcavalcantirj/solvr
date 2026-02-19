package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// ============================================================================
// POST /v1/questions/:id/answers - Create Answer with Embedding Tests
// ============================================================================

// TestCreateAnswer_WithEmbeddingService tests that embedding is generated on answer creation.
func TestCreateAnswer_WithEmbeddingService(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.1, 0.2, 0.3},
	}
	handler := NewQuestionsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := map[string]interface{}{
		"content": "This is a test answer with sufficient content length to be a valid answer.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}

	// Verify embedding text is the answer content
	expectedContent := "This is a test answer with sufficient content length to be a valid answer."
	if mockEmbed.lastInput != expectedContent {
		t.Errorf("expected embedding input to be answer content, got: %s", mockEmbed.lastInput)
	}

	// Verify the answer has embedding set
	if repo.createdAnswer == nil {
		t.Fatal("expected created answer to be set in repo")
	}
	if repo.createdAnswer.EmbeddingStr == nil {
		t.Error("expected embedding string to be set on created answer")
	}
}

// TestCreateAnswer_EmbeddingServiceFailure tests graceful degradation on embedding error.
func TestCreateAnswer_EmbeddingServiceFailure(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	mockEmbed := &MockEmbeddingService{
		err: fmt.Errorf("embedding API unavailable"),
	}
	handler := NewQuestionsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := map[string]interface{}{
		"content": "This is a test answer with sufficient content length to be a valid answer.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	// Answer should still be created despite embedding failure
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 despite embedding failure, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}

	// Embedding should be nil on the answer
	if repo.createdAnswer.EmbeddingStr != nil {
		t.Error("expected nil embedding on failure")
	}
}

// TestCreateAnswer_NoEmbeddingService tests normal answer creation without embedding service.
func TestCreateAnswer_NoEmbeddingService(t *testing.T) {
	repo := NewMockQuestionsRepository()
	question := createTestQuestion("question-123", "Test Question")
	repo.SetQuestion(&question)

	handler := NewQuestionsHandler(repo) // No embedding service set

	body := map[string]interface{}{
		"content": "This is a test answer with sufficient content length to be a valid answer.",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/questions/question-123/answers", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "question-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateAnswer(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdAnswer.EmbeddingStr != nil {
		t.Error("expected nil embedding when no service set")
	}
}

// ============================================================================
// PATCH /v1/answers/:id - Update Answer with Embedding Tests
// ============================================================================

// TestUpdateAnswer_ContentChangeRegeneratesEmbedding tests embedding regeneration on content change.
func TestUpdateAnswer_ContentChangeRegeneratesEmbedding(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.4, 0.5, 0.6},
	}
	handler := NewQuestionsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	newContent := "Updated answer content that is long enough to be valid content for an answer."
	body := map[string]interface{}{
		"content": newContent,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/answer-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user") // Same as answer author
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding regenerated on content change, called %d times", mockEmbed.callCount)
	}

	// Verify the embedding text is the new content
	if !strings.Contains(mockEmbed.lastInput, newContent) {
		t.Errorf("expected new content in embedding input, got: %s", mockEmbed.lastInput)
	}

	if repo.updatedAnswer == nil || repo.updatedAnswer.EmbeddingStr == nil {
		t.Error("expected embedding set on updated answer")
	}
}

// TestUpdateAnswer_EmbeddingFailureStillUpdates tests graceful degradation on update.
func TestUpdateAnswer_EmbeddingFailureStillUpdates(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	mockEmbed := &MockEmbeddingService{
		err: fmt.Errorf("embedding API unavailable"),
	}
	handler := NewQuestionsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	newContent := "Updated answer content that is long enough to be valid content for an answer."
	body := map[string]interface{}{
		"content": newContent,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/answer-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	// Answer should still be updated despite embedding failure
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 despite embedding failure, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}
}

// TestUpdateAnswer_NoContentChangeNoEmbedding tests no embedding when content is not changed.
func TestUpdateAnswer_NoContentChangeNoEmbedding(t *testing.T) {
	repo := NewMockQuestionsRepository()
	answer := createTestAnswer("answer-123", "question-123")
	repo.SetAnswer(&answer)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.1, 0.2, 0.3},
	}
	handler := NewQuestionsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	// Empty update body - no content change
	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/answers/answer-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "answer-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addQuestionsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.UpdateAnswer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 0 {
		t.Errorf("expected no embedding regeneration for no content change, called %d times", mockEmbed.callCount)
	}
}

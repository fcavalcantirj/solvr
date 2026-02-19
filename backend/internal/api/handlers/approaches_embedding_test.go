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
// POST /v1/problems/:id/approaches - Create Approach with Embedding Tests
// ============================================================================

// TestCreateApproach_WithEmbeddingService tests that embedding is generated on approach creation.
func TestCreateApproach_WithEmbeddingService(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem Title")
	repo.SetPost(&problem)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.1, 0.2, 0.3},
	}
	handler := NewProblemsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := map[string]interface{}{
		"angle":  "Test approach angle with sufficient length",
		"method": "Using goroutines and channels for concurrency",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems/problem-123/approaches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateApproach(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}

	// Verify embedding text combines angle + method
	if !strings.Contains(mockEmbed.lastInput, "Test approach angle with sufficient length") {
		t.Errorf("expected angle in embedding input, got: %s", mockEmbed.lastInput)
	}
	if !strings.Contains(mockEmbed.lastInput, "Using goroutines and channels for concurrency") {
		t.Errorf("expected method in embedding input, got: %s", mockEmbed.lastInput)
	}

	// Verify the approach has embedding set
	if repo.createdApproach == nil {
		t.Fatal("expected created approach to be set in repo")
	}
	if repo.createdApproach.EmbeddingStr == nil {
		t.Error("expected embedding string to be set on created approach")
	}
}

// TestCreateApproach_EmbeddingServiceFailure tests graceful degradation on embedding error.
func TestCreateApproach_EmbeddingServiceFailure(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem Title")
	repo.SetPost(&problem)

	mockEmbed := &MockEmbeddingService{
		err: fmt.Errorf("embedding API unavailable"),
	}
	handler := NewProblemsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := map[string]interface{}{
		"angle":  "Test approach angle with sufficient length",
		"method": "Test method for the approach",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems/problem-123/approaches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateApproach(w, req)

	// Approach should still be created despite embedding failure
	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201 despite embedding failure, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}

	// Embedding should be nil on the approach
	if repo.createdApproach.EmbeddingStr != nil {
		t.Error("expected nil embedding on failure")
	}
}

// TestCreateApproach_NoEmbeddingService tests normal approach creation without embedding service.
func TestCreateApproach_NoEmbeddingService(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem Title")
	repo.SetPost(&problem)

	handler := NewProblemsHandler(repo) // No embedding service set

	body := map[string]interface{}{
		"angle":  "Test approach angle with sufficient length",
		"method": "Test method for the approach",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems/problem-123/approaches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateApproach(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	if repo.createdApproach.EmbeddingStr != nil {
		t.Error("expected nil embedding when no service set")
	}
}

// ============================================================================
// PATCH /v1/approaches/:id - Update Approach with Embedding Tests
// ============================================================================

// TestUpdateApproach_MethodChangeRegeneratesEmbedding tests embedding regeneration on method change.
func TestUpdateApproach_MethodChangeRegeneratesEmbedding(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.4, 0.5, 0.6},
	}
	handler := NewProblemsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	newMethod := "Updated method using a completely different technique"
	body := map[string]interface{}{
		"method": newMethod,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/approaches/approach-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user") // Same as approach author
	w := httptest.NewRecorder()

	handler.UpdateApproach(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding regenerated on method change, called %d times", mockEmbed.callCount)
	}

	// Verify the embedding text includes the new method
	if !strings.Contains(mockEmbed.lastInput, newMethod) {
		t.Errorf("expected new method in embedding input, got: %s", mockEmbed.lastInput)
	}

	if repo.updatedApproach == nil || repo.updatedApproach.EmbeddingStr == nil {
		t.Error("expected embedding set on updated approach")
	}
}

// TestUpdateApproach_EmbeddingFailureStillUpdates tests graceful degradation on update.
func TestUpdateApproach_EmbeddingFailureStillUpdates(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	mockEmbed := &MockEmbeddingService{
		err: fmt.Errorf("embedding API unavailable"),
	}
	handler := NewProblemsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	newMethod := "Updated method that should still be saved"
	body := map[string]interface{}{
		"method": newMethod,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/approaches/approach-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.UpdateApproach(w, req)

	// Approach should still be updated despite embedding failure
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200 despite embedding failure, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 1 {
		t.Errorf("expected embedding service called once, got %d", mockEmbed.callCount)
	}
}

// TestUpdateApproach_StatusOnlyNoEmbeddingRegeneration tests no embedding when only status changes.
func TestUpdateApproach_StatusOnlyNoEmbeddingRegeneration(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	mockEmbed := &MockEmbeddingService{
		embedding: []float32{0.1, 0.2, 0.3},
	}
	handler := NewProblemsHandler(repo)
	handler.SetEmbeddingService(mockEmbed)

	body := map[string]interface{}{
		"status": "working",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/approaches/approach-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.UpdateApproach(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if mockEmbed.callCount != 0 {
		t.Errorf("expected no embedding regeneration for status-only change, called %d times", mockEmbed.callCount)
	}
}

// Package handlers contains HTTP request handlers for the Solvr API.
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

// TestCreateResponse_Success tests successful response creation.
func TestCreateResponse_Success(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"content":       "This is a thoughtful response to the idea with enough content.",
		"response_type": "build",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateResponse(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	if data["id"] == nil {
		t.Error("expected response id in response")
	}
}

// TestCreateResponse_NoAuth tests 401 when not authenticated.
func TestCreateResponse_NoAuth(t *testing.T) {
	repo := NewMockIdeasRepository()
	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"content":       "Test response content",
		"response_type": "build",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.CreateResponse(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreateResponse_IdeaNotFound tests 404 when idea doesn't exist.
func TestCreateResponse_IdeaNotFound(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdea(nil) // No idea found

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"content":       "Test response content",
		"response_type": "build",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/nonexistent/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateResponse(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestCreateResponse_InvalidType tests validation for invalid response type.
func TestCreateResponse_InvalidType(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"content":       "Test response content",
		"response_type": "invalid_type",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateResponse(w, req)

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
}

// TestCreateResponse_MissingContent tests validation for missing content.
func TestCreateResponse_MissingContent(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"content":       "",
		"response_type": "build",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/responses", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateResponse(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreateResponse_AllResponseTypes tests that all valid response types work.
func TestCreateResponse_AllResponseTypes(t *testing.T) {
	validTypes := []string{"build", "critique", "expand", "question", "support"}

	for _, rt := range validTypes {
		t.Run(rt, func(t *testing.T) {
			repo := NewMockIdeasRepository()
			idea := createTestIdea("idea-123", "Test Idea")
			repo.SetIdea(&idea)

			handler := NewIdeasHandler(repo)

			body := map[string]interface{}{
				"content":       "Test response content for " + rt,
				"response_type": rt,
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/responses", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "idea-123")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			req = addIdeasAuthContext(req, "user-456", "user")
			w := httptest.NewRecorder()

			handler.CreateResponse(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status 201 for type %s, got %d", rt, w.Code)
			}
		})
	}
}

// TestValidResponseTypes tests the IsValidResponseType helper.
func TestValidResponseTypes(t *testing.T) {
	valid := []models.ResponseType{
		models.ResponseTypeBuild,
		models.ResponseTypeCritique,
		models.ResponseTypeExpand,
		models.ResponseTypeQuestion,
		models.ResponseTypeSupport,
	}

	for _, rt := range valid {
		if !models.IsValidResponseType(rt) {
			t.Errorf("expected %s to be valid", rt)
		}
	}

	invalid := []models.ResponseType{"invalid", "random", ""}
	for _, rt := range invalid {
		if models.IsValidResponseType(rt) {
			t.Errorf("expected %s to be invalid", rt)
		}
	}
}

// TestEvolve_Success tests successful idea evolution linking.
func TestEvolve_Success(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"evolved_post_id": "evolved-post-id",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/evolve", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Evolve(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["idea_id"] != "idea-123" {
		t.Errorf("expected idea_id 'idea-123', got %v", resp["idea_id"])
	}
	if resp["evolved_post_id"] != "evolved-post-id" {
		t.Errorf("expected evolved_post_id 'evolved-post-id', got %v", resp["evolved_post_id"])
	}
	if repo.evolvedPostID != "idea-123" {
		t.Error("expected AddEvolvedInto to be called with correct idea ID")
	}
	if repo.evolvedIntoID != "evolved-post-id" {
		t.Error("expected AddEvolvedInto to be called with correct evolved post ID")
	}
}

// TestEvolve_NoAuth tests 401 when not authenticated.
func TestEvolve_NoAuth(t *testing.T) {
	repo := NewMockIdeasRepository()
	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"evolved_post_id": "evolved-post-id",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/evolve", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Evolve(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestEvolve_IdeaNotFound tests 404 when idea doesn't exist.
func TestEvolve_IdeaNotFound(t *testing.T) {
	repo := NewMockIdeasRepository()
	repo.SetIdea(nil) // No idea found

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"evolved_post_id": "evolved-post-id",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/nonexistent/evolve", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Evolve(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// TestEvolve_MissingEvolvedPostID tests validation for missing evolved_post_id.
func TestEvolve_MissingEvolvedPostID(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"evolved_post_id": "",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/evolve", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Evolve(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestEvolve_EvolvedPostNotFound tests 404 when evolved post doesn't exist.
func TestEvolve_EvolvedPostNotFound(t *testing.T) {
	repo := NewMockIdeasRepository()
	idea := createTestIdea("idea-123", "Test Idea")
	repo.SetIdea(&idea)

	handler := NewIdeasHandler(repo)

	body := map[string]interface{}{
		"evolved_post_id": "nonexistent-post",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/ideas/idea-123/evolve", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "idea-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addIdeasAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.Evolve(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	errObj := resp["error"].(map[string]interface{})
	if errObj["message"] != "evolved post not found" {
		t.Errorf("expected message 'evolved post not found', got %v", errObj["message"])
	}
}

// TestResponse_VoteScore tests the VoteScore method.
func TestResponse_VoteScore(t *testing.T) {
	r := &models.Response{
		Upvotes:   10,
		Downvotes: 3,
	}
	if r.VoteScore() != 7 {
		t.Errorf("expected vote score 7, got %d", r.VoteScore())
	}
}

// TestValidResponseTypes_All tests ValidResponseTypes returns all types.
func TestValidResponseTypes_All(t *testing.T) {
	types := models.ValidResponseTypes()
	if len(types) != 5 {
		t.Errorf("expected 5 response types, got %d", len(types))
	}
	expected := map[models.ResponseType]bool{
		models.ResponseTypeBuild:    true,
		models.ResponseTypeCritique: true,
		models.ResponseTypeExpand:   true,
		models.ResponseTypeQuestion: true,
		models.ResponseTypeSupport:  true,
	}
	for _, rt := range types {
		if !expected[rt] {
			t.Errorf("unexpected response type: %s", rt)
		}
	}
}

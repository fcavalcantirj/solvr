// Package handlers contains HTTP request handlers for the Solvr API.
// This file contains tests for approach-related endpoints.
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

// ============================================================================
// GET /v1/problems/:id/approaches - List Approaches Tests
// ============================================================================

// TestListApproaches_Success tests successful listing of approaches.
func TestListApproaches_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)
	repo.SetApproaches([]models.ApproachWithAuthor{
		createTestApproach("approach-1", "problem-123"),
		createTestApproach("approach-2", "problem-123"),
	})

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/approaches", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.ListApproaches(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}

	if len(data) != 2 {
		t.Errorf("expected 2 approaches, got %d", len(data))
	}
}

// TestListApproaches_ProblemNotFound tests 404 when problem doesn't exist.
func TestListApproaches_ProblemNotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPost(nil)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/nonexistent/approaches", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.ListApproaches(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/problems/:id/approaches - Create Approach Tests
// ============================================================================

// TestCreateApproach_Success tests successful approach creation.
func TestCreateApproach_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"angle":       "Try using transactions for atomicity",
		"method":      "Wrap all database operations in a single transaction",
		"assumptions": []string{"Database supports transactions"},
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
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	// Verify approach was created with correct data
	if repo.createdApproach.ProblemID != "problem-123" {
		t.Errorf("expected problem_id 'problem-123', got '%s'", repo.createdApproach.ProblemID)
	}

	if repo.createdApproach.Status != models.ApproachStatusStarting {
		t.Errorf("expected status 'starting', got '%s'", repo.createdApproach.Status)
	}
}

// TestCreateApproach_NoAuth tests 401 when not authenticated.
func TestCreateApproach_NoAuth(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"angle": "Test approach",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems/problem-123/approaches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.CreateApproach(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestCreateApproach_MissingAngle tests 400 when angle is missing.
func TestCreateApproach_MissingAngle(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"method": "Some method", // Missing angle
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestCreateApproach_ProblemNotFound tests 404 when problem doesn't exist.
func TestCreateApproach_ProblemNotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPost(nil)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"angle": "Test approach",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/problems/nonexistent/approaches", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.CreateApproach(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// PATCH /v1/approaches/:id - Update Approach Tests
// ============================================================================

// TestUpdateApproach_Success tests successful approach update.
func TestUpdateApproach_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"status":  "working",
		"outcome": "Making progress on the solution",
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
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	if repo.updatedApproach.Status != models.ApproachStatusWorking {
		t.Errorf("expected status 'working', got '%s'", repo.updatedApproach.Status)
	}
}

// TestUpdateApproach_NotAuthor tests 403 for non-author.
func TestUpdateApproach_NotAuthor(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"status": "working",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/approaches/approach-123", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "other-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.UpdateApproach(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestUpdateApproach_InvalidStatus tests 400 for invalid status.
func TestUpdateApproach_InvalidStatus(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"status": "invalid-status",
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestUpdateApproach_NotFound tests 404 when approach doesn't exist.
func TestUpdateApproach_NotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetApproach(nil)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"status": "working",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/v1/approaches/nonexistent", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.UpdateApproach(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/approaches/:id/progress - Add Progress Note Tests
// ============================================================================

// TestAddProgressNote_Success tests successful progress note creation.
func TestAddProgressNote_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"content": "Made good progress today, identified the root cause",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/progress", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user") // Same as approach author
	w := httptest.NewRecorder()

	handler.AddProgressNote(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d; body: %s", w.Code, w.Body.String())
	}

	if repo.progressNote.ApproachID != "approach-123" {
		t.Errorf("expected approach_id 'approach-123', got '%s'", repo.progressNote.ApproachID)
	}
}

// TestAddProgressNote_NotAuthor tests 403 for non-author.
func TestAddProgressNote_NotAuthor(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"content": "Progress note",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/progress", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "other-user", "user") // Different user
	w := httptest.NewRecorder()

	handler.AddProgressNote(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestAddProgressNote_MissingContent tests 400 when content is missing.
func TestAddProgressNote_MissingContent(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-123", "problem-123")
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/progress", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-456", "user")
	w := httptest.NewRecorder()

	handler.AddProgressNote(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ============================================================================
// POST /v1/approaches/:id/verify - Verify Approach Tests
// ============================================================================

// TestVerifyApproach_Success tests successful approach verification.
func TestVerifyApproach_Success(t *testing.T) {
	repo := NewMockProblemsRepository()

	// Create problem owned by user-123
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	// Create approach with status succeeded
	approach := createTestApproach("approach-123", "problem-123")
	approach.Status = models.ApproachStatusSucceeded
	approach.Solution = "The solution that worked"
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"verified": true,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/verify", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-123", "user") // Problem owner
	w := httptest.NewRecorder()

	handler.VerifyApproach(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestVerifyApproach_NotProblemOwner tests 403 for non-problem owner.
func TestVerifyApproach_NotProblemOwner(t *testing.T) {
	repo := NewMockProblemsRepository()

	// Create problem owned by user-123
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	// Create approach
	approach := createTestApproach("approach-123", "problem-123")
	approach.Status = models.ApproachStatusSucceeded
	repo.SetApproach(&approach)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"verified": true,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/verify", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "other-user", "user") // Not the problem owner
	w := httptest.NewRecorder()

	handler.VerifyApproach(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestVerifyApproach_NoAuth tests 401 when not authenticated.
func TestVerifyApproach_NoAuth(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"verified": true,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/approach-123/verify", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "approach-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	// No auth context
	w := httptest.NewRecorder()

	handler.VerifyApproach(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// TestVerifyApproach_ApproachNotFound tests 404 when approach doesn't exist.
func TestVerifyApproach_ApproachNotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetApproach(nil)

	handler := NewProblemsHandler(repo)

	body := map[string]interface{}{
		"verified": true,
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/approaches/nonexistent/verify", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req = addProblemsAuthContext(req, "user-123", "user")
	w := httptest.NewRecorder()

	handler.VerifyApproach(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// ============================================================================
// GET /v1/problems/:id/approaches/:approachId/history - Version History Tests
// ============================================================================

// MockRelationshipsRepository implements ApproachRelationshipsRepositoryInterface for testing.
type MockRelationshipsRepository struct {
	history *models.ApproachVersionHistory
	err     error
}

func (m *MockRelationshipsRepository) GetVersionChain(_ context.Context, _ string, _ int) (*models.ApproachVersionHistory, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.history, nil
}

func (m *MockRelationshipsRepository) CreateRelationship(_ context.Context, rel *models.ApproachRelationship) (*models.ApproachRelationship, error) {
	if m.err != nil {
		return nil, m.err
	}
	rel.ID = "rel-mock-1"
	rel.CreatedAt = time.Now()
	return rel, nil
}

// TestGetApproachHistory_Success tests a 3-version chain.
func TestGetApproachHistory_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-v3", "problem-123")
	repo.SetApproach(&approach)

	relRepo := &MockRelationshipsRepository{
		history: &models.ApproachVersionHistory{
			Current: approach,
			History: []models.ApproachWithAuthor{
				createTestApproach("approach-v1", "problem-123"),
				createTestApproach("approach-v2", "problem-123"),
			},
			Relationships: []models.ApproachRelationship{
				{ID: "rel-1", FromApproachID: "approach-v3", ToApproachID: "approach-v2", RelationType: models.RelationTypeUpdates},
				{ID: "rel-2", FromApproachID: "approach-v2", ToApproachID: "approach-v1", RelationType: models.RelationTypeUpdates},
			},
		},
	}

	handler := NewProblemsHandler(repo)
	handler.SetApproachRelationshipsRepository(relRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/approaches/approach-v3/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	rctx.URLParams.Add("approachId", "approach-v3")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetApproachHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	history, ok := data["history"].([]interface{})
	if !ok {
		t.Fatal("expected history array in data")
	}
	if len(history) != 2 {
		t.Errorf("expected 2 history entries, got %d", len(history))
	}

	rels, ok := data["relationships"].([]interface{})
	if !ok {
		t.Fatal("expected relationships array in data")
	}
	if len(rels) != 2 {
		t.Errorf("expected 2 relationships, got %d", len(rels))
	}
}

// TestGetApproachHistory_NoHistory tests an approach with no version history.
func TestGetApproachHistory_NoHistory(t *testing.T) {
	repo := NewMockProblemsRepository()
	approach := createTestApproach("approach-1", "problem-123")
	repo.SetApproach(&approach)

	relRepo := &MockRelationshipsRepository{
		history: &models.ApproachVersionHistory{
			Current:       approach,
			History:       []models.ApproachWithAuthor{},
			Relationships: []models.ApproachRelationship{},
		},
	}

	handler := NewProblemsHandler(repo)
	handler.SetApproachRelationshipsRepository(relRepo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/approaches/approach-1/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	rctx.URLParams.Add("approachId", "approach-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetApproachHistory(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	history := data["history"].([]interface{})
	if len(history) != 0 {
		t.Errorf("expected 0 history entries, got %d", len(history))
	}
}

// TestGetApproachHistory_NotFound tests 404 when approach doesn't exist.
func TestGetApproachHistory_NotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetApproach(nil)

	handler := NewProblemsHandler(repo)
	handler.SetApproachRelationshipsRepository(&MockRelationshipsRepository{})

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/approaches/nonexistent/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	rctx.URLParams.Add("approachId", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.GetApproachHistory(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

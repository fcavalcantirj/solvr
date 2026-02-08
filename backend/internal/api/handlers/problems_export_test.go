// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
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
// GET /v1/problems/:id/export - Export Problem Tests
// ============================================================================

// TestExportProblem_Success tests successful export of a problem with approaches.
func TestExportProblem_Success(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem Title")
	problem.Description = "This is a detailed problem description that explains the issue we are trying to solve."
	problem.Tags = []string{"go", "testing", "api"}
	repo.SetPost(&problem)

	// Add approaches with different statuses
	approaches := []models.ApproachWithAuthor{
		createTestApproach("approach-1", "problem-123"),
		createTestApproach("approach-2", "problem-123"),
	}
	approaches[0].Angle = "First approach using method A"
	approaches[0].Method = "Use dependency injection"
	approaches[0].Status = models.ApproachStatusWorking
	approaches[1].Angle = "Second approach using method B"
	approaches[1].Method = "Use mocking"
	approaches[1].Status = models.ApproachStatusSucceeded
	repo.SetApproaches(approaches)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify markdown contains expected sections
	if !strings.Contains(resp.Markdown, "# Problem: Test Problem Title") {
		t.Error("expected problem title in markdown")
	}
	if !strings.Contains(resp.Markdown, "## Description") {
		t.Error("expected description section in markdown")
	}
	if !strings.Contains(resp.Markdown, "## Approaches (2)") {
		t.Error("expected approaches section with count in markdown")
	}
	if !strings.Contains(resp.Markdown, "## Summary") {
		t.Error("expected summary section in markdown")
	}
	if !strings.Contains(resp.Markdown, "https://solvr.dev/problems/problem-123") {
		t.Error("expected problem URL in markdown")
	}
	if !strings.Contains(resp.Markdown, "go, testing, api") {
		t.Error("expected tags in markdown")
	}

	// Verify token estimate is reasonable
	if resp.TokenEstimate <= 0 {
		t.Error("expected positive token estimate")
	}
	// Token estimate should be roughly len(markdown) / 4
	expectedEstimate := len(resp.Markdown) / 4
	if resp.TokenEstimate != expectedEstimate {
		t.Errorf("expected token estimate %d, got %d", expectedEstimate, resp.TokenEstimate)
	}
}

// TestExportProblem_NotFound tests 404 for non-existent problem.
func TestExportProblem_NotFound(t *testing.T) {
	repo := NewMockProblemsRepository()
	repo.SetPost(nil)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/nonexistent/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj := resp["error"].(map[string]interface{})
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error code NOT_FOUND, got %v", errObj["code"])
	}
}

// TestExportProblem_EmptyApproaches tests export with no approaches.
func TestExportProblem_EmptyApproaches(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)
	repo.SetApproaches([]models.ApproachWithAuthor{}) // Empty approaches

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Markdown, "## Approaches (0)") {
		t.Error("expected approaches section with 0 count")
	}
	if !strings.Contains(resp.Markdown, "Total approaches: 0") {
		t.Error("expected summary with 0 total approaches")
	}
}

// TestExportProblem_IncludesStatusSummary tests that the summary includes correct status counts.
func TestExportProblem_IncludesStatusSummary(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	// Create approaches with different statuses
	approaches := []models.ApproachWithAuthor{
		createTestApproach("approach-1", "problem-123"),
		createTestApproach("approach-2", "problem-123"),
		createTestApproach("approach-3", "problem-123"),
		createTestApproach("approach-4", "problem-123"),
	}
	approaches[0].Status = models.ApproachStatusSucceeded
	approaches[1].Status = models.ApproachStatusFailed
	approaches[2].Status = models.ApproachStatusWorking
	approaches[3].Status = models.ApproachStatusStarting
	repo.SetApproaches(approaches)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify summary stats
	if !strings.Contains(resp.Markdown, "Total approaches: 4") {
		t.Error("expected total approaches: 4")
	}
	if !strings.Contains(resp.Markdown, "Succeeded: 1") {
		t.Error("expected Succeeded: 1")
	}
	if !strings.Contains(resp.Markdown, "Failed: 1") {
		t.Error("expected Failed: 1")
	}
	if !strings.Contains(resp.Markdown, "In Progress: 2") {
		t.Error("expected In Progress: 2 (working + starting)")
	}
}

// TestExportProblem_IncludesProgressNotes tests that progress notes are included.
func TestExportProblem_IncludesProgressNotes(t *testing.T) {
	repo := NewMockProblemsRepositoryWithNotes()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	approach := createTestApproach("approach-1", "problem-123")
	approach.ProgressNotes = []models.ProgressNote{
		{
			ID:         "note-1",
			ApproachID: "approach-1",
			Content:    "First progress update with details",
			CreatedAt:  time.Now().Add(-24 * time.Hour),
		},
		{
			ID:         "note-2",
			ApproachID: "approach-1",
			Content:    "Second progress update",
			CreatedAt:  time.Now(),
		},
	}
	repo.SetApproaches([]models.ApproachWithAuthor{approach})
	repo.SetProgressNotes(approach.ProgressNotes)

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Markdown, "**Progress Notes:**") {
		t.Error("expected progress notes section")
	}
	if !strings.Contains(resp.Markdown, "First progress update with details") {
		t.Error("expected first progress note content")
	}
	if !strings.Contains(resp.Markdown, "Second progress update") {
		t.Error("expected second progress note content")
	}
}

// TestExportProblem_IncludesApproachMethod tests that approach method is included.
func TestExportProblem_IncludesApproachMethod(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	approach := createTestApproach("approach-1", "problem-123")
	approach.Method = "Use dependency injection pattern with interface abstractions"
	repo.SetApproaches([]models.ApproachWithAuthor{approach})

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Markdown, "**Method:**") {
		t.Error("expected method section")
	}
	if !strings.Contains(resp.Markdown, "Use dependency injection pattern") {
		t.Error("expected method content")
	}
}

// TestExportProblem_IncludesAssumptions tests that approach assumptions are included.
func TestExportProblem_IncludesAssumptions(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	approach := createTestApproach("approach-1", "problem-123")
	approach.Assumptions = []string{"Go version 1.21+", "PostgreSQL 14+", "Docker available"}
	repo.SetApproaches([]models.ApproachWithAuthor{approach})

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Markdown, "**Assumptions:**") {
		t.Error("expected assumptions section")
	}
	if !strings.Contains(resp.Markdown, "- Go version 1.21+") {
		t.Error("expected first assumption")
	}
	if !strings.Contains(resp.Markdown, "- PostgreSQL 14+") {
		t.Error("expected second assumption")
	}
}

// TestExportProblem_MissingID tests 400 for missing problem ID.
func TestExportProblem_MissingID(t *testing.T) {
	repo := NewMockProblemsRepository()
	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems//export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "") // Empty ID
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// TestExportProblem_IncludesOutcome tests that approach outcome is included when present.
func TestExportProblem_IncludesOutcome(t *testing.T) {
	repo := NewMockProblemsRepository()
	problem := createTestProblem("problem-123", "Test Problem")
	repo.SetPost(&problem)

	approach := createTestApproach("approach-1", "problem-123")
	approach.Status = models.ApproachStatusFailed
	approach.Outcome = "Failed due to memory constraints. Need to optimize algorithm."
	repo.SetApproaches([]models.ApproachWithAuthor{approach})

	handler := NewProblemsHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/problems/problem-123/export", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "problem-123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	handler.Export(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp ProblemExportResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !strings.Contains(resp.Markdown, "**Outcome:**") {
		t.Error("expected outcome section")
	}
	if !strings.Contains(resp.Markdown, "Failed due to memory constraints") {
		t.Error("expected outcome content")
	}
}

// MockProblemsRepositoryWithNotes extends MockProblemsRepository with progress notes support.
type MockProblemsRepositoryWithNotes struct {
	*MockProblemsRepository
	progressNotes []models.ProgressNote
}

func NewMockProblemsRepositoryWithNotes() *MockProblemsRepositoryWithNotes {
	return &MockProblemsRepositoryWithNotes{
		MockProblemsRepository: NewMockProblemsRepository(),
	}
}

func (m *MockProblemsRepositoryWithNotes) SetProgressNotes(notes []models.ProgressNote) {
	m.progressNotes = notes
}

func (m *MockProblemsRepositoryWithNotes) GetProgressNotes(ctx context.Context, approachID string) ([]models.ProgressNote, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.progressNotes, nil
}

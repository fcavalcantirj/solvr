package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// mockIncidentWriter implements IncidentWriter for testing.
type mockIncidentWriter struct {
	createCalled       bool
	updateStatusCalled bool
	addUpdateCalled    bool
	err                error
}

func (m *mockIncidentWriter) Create(ctx context.Context, incident models.Incident) error {
	m.createCalled = true
	return m.err
}

func (m *mockIncidentWriter) UpdateStatus(ctx context.Context, id string, status string) error {
	m.updateStatusCalled = true
	return m.err
}

func (m *mockIncidentWriter) AddUpdate(ctx context.Context, update models.IncidentUpdate) error {
	m.addUpdateCalled = true
	return m.err
}

func TestIncidentAdminHandler_CreateIncident(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{}
	handler := NewIncidentAdminHandler(writer)

	body := `{"id":"INC-2026-0001","title":"Test incident","severity":"minor","affected_services":["api"]}`
	req := httptest.NewRequest(http.MethodPost, "/admin/incidents", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.CreateIncident(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !writer.createCalled {
		t.Error("expected Create() to be called")
	}
}

func TestIncidentAdminHandler_CreateIncident_MissingAuth(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{}
	handler := NewIncidentAdminHandler(writer)

	body := `{"id":"INC-001","title":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/incidents", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.CreateIncident(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestIncidentAdminHandler_CreateIncident_MissingFields(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{}
	handler := NewIncidentAdminHandler(writer)

	body := `{"title":"Missing ID"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/incidents", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.CreateIncident(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestIncidentAdminHandler_CreateIncident_RepoError(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{err: errors.New("db error")}
	handler := NewIncidentAdminHandler(writer)

	body := `{"id":"INC-001","title":"Test"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/incidents", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-key")
	rec := httptest.NewRecorder()

	handler.CreateIncident(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestIncidentAdminHandler_UpdateIncidentStatus(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{}
	handler := NewIncidentAdminHandler(writer)

	body := `{"status":"resolved"}`
	req := httptest.NewRequest(http.MethodPatch, "/admin/incidents/INC-001", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-key")

	// Set chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "INC-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	handler.UpdateIncidentStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !writer.updateStatusCalled {
		t.Error("expected UpdateStatus() to be called")
	}

	var resp map[string]any
	json.NewDecoder(rec.Body).Decode(&resp)
	data := resp["data"].(map[string]any)
	if data["status"] != "resolved" {
		t.Errorf("expected status 'resolved', got '%v'", data["status"])
	}
}

func TestIncidentAdminHandler_AddIncidentUpdate(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{}
	handler := NewIncidentAdminHandler(writer)

	body := `{"status":"identified","message":"Root cause found"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/incidents/INC-001/updates", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-key")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "INC-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	handler.AddIncidentUpdate(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if !writer.addUpdateCalled {
		t.Error("expected AddUpdate() to be called")
	}
}

func TestIncidentAdminHandler_AddIncidentUpdate_MissingFields(t *testing.T) {
	t.Setenv("ADMIN_API_KEY", "test-key")

	writer := &mockIncidentWriter{}
	handler := NewIncidentAdminHandler(writer)

	body := `{"message":"Missing status field"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/incidents/INC-001/updates", bytes.NewBufferString(body))
	req.Header.Set("X-Admin-API-Key", "test-key")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "INC-001")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()

	handler.AddIncidentUpdate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

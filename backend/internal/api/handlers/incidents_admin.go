package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// IncidentWriter handles incident create/update operations.
type IncidentWriter interface {
	Create(ctx context.Context, incident models.Incident) error
	UpdateStatus(ctx context.Context, id string, status string) error
	AddUpdate(ctx context.Context, update models.IncidentUpdate) error
}

// IncidentAdminHandler handles admin incident management.
type IncidentAdminHandler struct {
	repo IncidentWriter
}

// NewIncidentAdminHandler creates a new IncidentAdminHandler.
func NewIncidentAdminHandler(repo IncidentWriter) *IncidentAdminHandler {
	return &IncidentAdminHandler{repo: repo}
}

type createIncidentRequest struct {
	ID               string   `json:"id"`
	Title            string   `json:"title"`
	Severity         string   `json:"severity"`
	AffectedServices []string `json:"affected_services"`
}

type updateIncidentStatusRequest struct {
	Status string `json:"status"`
}

type addIncidentUpdateRequest struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CreateIncident handles POST /admin/incidents.
func (h *IncidentAdminHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	if !checkIncidentAdminAuth(w, r) {
		return
	}

	var req createIncidentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIncidentAdminError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}

	if req.ID == "" || req.Title == "" {
		writeIncidentAdminError(w, http.StatusBadRequest, "MISSING_FIELDS", "id and title are required")
		return
	}

	severity := models.IncidentSeverityMinor
	if req.Severity != "" {
		severity = models.IncidentSeverity(req.Severity)
	}

	incident := models.Incident{
		ID:               req.ID,
		Title:            req.Title,
		Status:           models.IncidentStatusInvestigating,
		Severity:         severity,
		AffectedServices: req.AffectedServices,
	}

	if err := h.repo.Create(r.Context(), incident); err != nil {
		writeIncidentAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create incident")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]string{
			"id":     req.ID,
			"status": "created",
		},
	})
}

// UpdateIncidentStatus handles PATCH /admin/incidents/{id}.
func (h *IncidentAdminHandler) UpdateIncidentStatus(w http.ResponseWriter, r *http.Request) {
	if !checkIncidentAdminAuth(w, r) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeIncidentAdminError(w, http.StatusBadRequest, "MISSING_ID", "incident ID required")
		return
	}

	var req updateIncidentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIncidentAdminError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}

	if req.Status == "" {
		writeIncidentAdminError(w, http.StatusBadRequest, "MISSING_STATUS", "status is required")
		return
	}

	if err := h.repo.UpdateStatus(r.Context(), id, req.Status); err != nil {
		writeIncidentAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update incident")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]string{
			"id":     id,
			"status": req.Status,
		},
	})
}

// AddIncidentUpdate handles POST /admin/incidents/{id}/updates.
func (h *IncidentAdminHandler) AddIncidentUpdate(w http.ResponseWriter, r *http.Request) {
	if !checkIncidentAdminAuth(w, r) {
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeIncidentAdminError(w, http.StatusBadRequest, "MISSING_ID", "incident ID required")
		return
	}

	var req addIncidentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeIncidentAdminError(w, http.StatusBadRequest, "INVALID_JSON", "invalid JSON body")
		return
	}

	if req.Status == "" || req.Message == "" {
		writeIncidentAdminError(w, http.StatusBadRequest, "MISSING_FIELDS", "status and message are required")
		return
	}

	update := models.IncidentUpdate{
		IncidentID: id,
		Status:     models.IncidentStatus(req.Status),
		Message:    req.Message,
	}

	if err := h.repo.AddUpdate(r.Context(), update); err != nil {
		writeIncidentAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to add incident update")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"data": map[string]string{
			"incident_id": id,
			"status":      "update_added",
		},
	})
}

// checkIncidentAdminAuth validates X-Admin-API-Key header.
func checkIncidentAdminAuth(w http.ResponseWriter, r *http.Request) bool {
	adminKey := os.Getenv("ADMIN_API_KEY")
	if adminKey == "" {
		writeIncidentAdminError(w, http.StatusServiceUnavailable, "ADMIN_NOT_CONFIGURED", "admin API key not configured")
		return false
	}
	providedKey := r.Header.Get("X-Admin-API-Key")
	if providedKey == "" {
		writeIncidentAdminError(w, http.StatusUnauthorized, "MISSING_API_KEY", "X-Admin-API-Key header required")
		return false
	}
	if providedKey != adminKey {
		writeIncidentAdminError(w, http.StatusForbidden, "INVALID_API_KEY", "invalid admin API key")
		return false
	}
	return true
}

func writeIncidentAdminError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

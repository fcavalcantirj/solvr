// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ReportsRepositoryInterface defines the database operations for reports.
type ReportsRepositoryInterface interface {
	Create(ctx context.Context, report *models.Report) (*models.Report, error)
	HasReported(ctx context.Context, targetType models.ReportTargetType, targetID, reporterType, reporterID string) (bool, error)
}

// ReportsHandler handles report HTTP requests.
type ReportsHandler struct {
	repo   ReportsRepositoryInterface
	logger *slog.Logger
}

// NewReportsHandler creates a new ReportsHandler.
func NewReportsHandler(repo ReportsRepositoryInterface) *ReportsHandler {
	return &ReportsHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger sets a custom logger for the handler.
func (h *ReportsHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// CreateReportRequest is the request body for creating a report.
type CreateReportRequest struct {
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Reason     string `json:"reason"`
	Details    string `json:"details,omitempty"`
}

// Create handles POST /v1/reports - create a new report.
func (h *ReportsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Get auth info (must be authenticated)
	authInfo := getAuthInfo(r)
	if authInfo == nil {
		writeReportsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeReportsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}

	// Validate target type
	targetType := models.ReportTargetType(req.TargetType)
	if !models.IsValidReportTargetType(targetType) {
		writeReportsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid target_type: must be post, answer, approach, response, or comment")
		return
	}

	// Validate target ID
	if req.TargetID == "" {
		writeReportsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_id is required")
		return
	}

	// Validate reason
	reason := models.ReportReason(req.Reason)
	if !models.IsValidReportReason(reason) {
		writeReportsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid reason: must be spam, offensive, off_topic, misleading, or other")
		return
	}

	// Create report
	report := &models.Report{
		TargetType:   targetType,
		TargetID:     req.TargetID,
		ReporterType: authInfo.authorType,
		ReporterID:   authInfo.authorID,
		Reason:       reason,
		Details:      req.Details,
	}

	created, err := h.repo.Create(r.Context(), report)
	if err != nil {
		if err == db.ErrReportExists {
			writeReportsError(w, http.StatusConflict, "ALREADY_REPORTED", "you have already reported this content")
			return
		}
		ctx := response.LogContext{
			Operation: "CreateReport",
			Resource:  "report",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"targetType": req.TargetType, "targetID": req.TargetID},
		}
		response.WriteInternalErrorWithLog(w, "failed to create report", err, ctx, h.logger)
		return
	}

	writeReportsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": map[string]interface{}{
			"id":          created.ID,
			"target_type": created.TargetType,
			"target_id":   created.TargetID,
			"reason":      created.Reason,
			"status":      created.Status,
			"created_at":  created.CreatedAt,
		},
	})
}

// Check handles GET /v1/reports/check - check if user has reported content.
func (h *ReportsHandler) Check(w http.ResponseWriter, r *http.Request) {
	// Get auth info (must be authenticated)
	authInfo := getAuthInfo(r)
	if authInfo == nil {
		writeReportsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	targetType := r.URL.Query().Get("target_type")
	targetID := r.URL.Query().Get("target_id")

	if targetType == "" || targetID == "" {
		writeReportsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_type and target_id are required")
		return
	}

	// Validate target type
	reportTargetType := models.ReportTargetType(targetType)
	if !models.IsValidReportTargetType(reportTargetType) {
		writeReportsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid target_type")
		return
	}

	hasReported, err := h.repo.HasReported(r.Context(), reportTargetType, targetID, string(authInfo.authorType), authInfo.authorID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "CheckReport",
			Resource:  "report",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to check report", err, ctx, h.logger)
		return
	}

	writeReportsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"reported": hasReported,
		},
	})
}

// writeReportsJSON writes a JSON response.
func writeReportsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeReportsError writes an error JSON response.
func writeReportsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

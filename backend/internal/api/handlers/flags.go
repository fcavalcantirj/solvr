// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// FlagsRepositoryInterface defines the database operations for flags.
type FlagsRepositoryInterface interface {
	// CreateFlag creates a new flag record.
	CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error)

	// TargetExists checks if the target (post, comment, answer, approach, response) exists.
	TargetExists(ctx context.Context, targetType, targetID string) (bool, error)

	// FlagExists checks if a flag already exists from the same reporter for the same target.
	FlagExists(ctx context.Context, targetType, targetID, reporterType, reporterID string) (bool, error)
}

// FlagsHandler handles flag-related HTTP requests.
type FlagsHandler struct {
	repo FlagsRepositoryInterface
}

// NewFlagsHandler creates a new FlagsHandler.
func NewFlagsHandler(repo FlagsRepositoryInterface) *FlagsHandler {
	return &FlagsHandler{repo: repo}
}

// CreateFlagRequest is the request body for creating a flag.
type CreateFlagRequest struct {
	TargetType string `json:"target_type"` // post, comment, answer, approach, response
	TargetID   string `json:"target_id"`   // UUID of the target
	Reason     string `json:"reason"`      // spam, offensive, duplicate, incorrect, low_quality, other
	Details    string `json:"details,omitempty"`
}

// Create handles POST /v1/flags - create a flag for content.
func (h *FlagsHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeFlagsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	// Parse request body
	var req CreateFlagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate target_type
	targetType := strings.TrimSpace(req.TargetType)
	if targetType == "" {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_type is required")
		return
	}
	if !isValidFlagTargetType(targetType) {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"invalid target_type, must be one of: post, comment, answer, approach, response")
		return
	}

	// Validate target_id
	targetID := strings.TrimSpace(req.TargetID)
	if targetID == "" {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_id is required")
		return
	}
	if _, err := uuid.Parse(targetID); err != nil {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_id must be a valid UUID")
		return
	}

	// Validate reason
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "reason is required")
		return
	}
	if !models.IsValidFlagReason(reason) {
		writeFlagsError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"invalid reason, must be one of: spam, offensive, duplicate, incorrect, low_quality, other")
		return
	}

	// Determine reporter type and ID
	reporterType := "human"
	reporterID := claims.UserID
	if claims.Role == "agent" {
		reporterType = "agent"
	}

	// Check if target exists
	exists, err := h.repo.TargetExists(r.Context(), targetType, targetID)
	if err != nil {
		writeFlagsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify target")
		return
	}
	if !exists {
		writeFlagsError(w, http.StatusNotFound, "NOT_FOUND", "target not found")
		return
	}

	// Check for duplicate flag
	duplicate, err := h.repo.FlagExists(r.Context(), targetType, targetID, reporterType, reporterID)
	if err != nil {
		writeFlagsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check for duplicate flag")
		return
	}
	if duplicate {
		writeFlagsError(w, http.StatusConflict, "DUPLICATE_FLAG", "you have already flagged this content")
		return
	}

	// Parse target_id as UUID
	targetUUID, _ := uuid.Parse(targetID)

	// Create flag
	flag := &models.Flag{
		TargetType:   targetType,
		TargetID:     targetUUID,
		ReporterType: reporterType,
		ReporterID:   reporterID,
		Reason:       reason,
		Details:      strings.TrimSpace(req.Details),
		Status:       "pending",
	}

	createdFlag, err := h.repo.CreateFlag(r.Context(), flag)
	if err != nil {
		writeFlagsError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create flag")
		return
	}

	writeFlagsJSON(w, http.StatusCreated, map[string]interface{}{
		"data": createdFlag,
	})
}

// isValidFlagTargetType checks if a target type is valid for flagging.
func isValidFlagTargetType(targetType string) bool {
	for _, t := range models.ValidFlagTargetTypes {
		if t == targetType {
			return true
		}
	}
	return false
}

// writeFlagsJSON writes a JSON response.
func writeFlagsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeFlagsError writes an error JSON response.
func writeFlagsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

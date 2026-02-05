// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/go-chi/chi/v5"
)

// ViewsRepositoryInterface defines the database operations for view tracking.
type ViewsRepositoryInterface interface {
	RecordView(ctx context.Context, postID, viewerType, viewerID string) (int, error)
	RecordAnonymousView(ctx context.Context, postID, sessionID string) (int, error)
	GetViewCount(ctx context.Context, postID string) (int, error)
}

// ViewsHandler handles view tracking HTTP requests.
type ViewsHandler struct {
	repo   ViewsRepositoryInterface
	logger *slog.Logger
}

// NewViewsHandler creates a new ViewsHandler.
func NewViewsHandler(repo ViewsRepositoryInterface) *ViewsHandler {
	return &ViewsHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger sets a custom logger for the handler.
func (h *ViewsHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// RecordView handles POST /v1/posts/:id/view - record a view on a post.
// Authenticated users' views are tracked by user ID.
// Anonymous views can be tracked by session ID.
func (h *ViewsHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		writeViewsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	var viewerType, viewerID string

	// Check for authenticated user
	authInfo := getAuthInfo(r)
	if authInfo != nil {
		viewerType = string(authInfo.authorType)
		viewerID = authInfo.authorID
	} else {
		// For anonymous views, use session ID from header or generate one
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			// Allow anonymous view without session tracking
			// Use a placeholder that will be unique per request
			sessionID = r.Header.Get("X-Request-ID")
			if sessionID == "" {
				sessionID = "anonymous"
			}
		}
		viewerType = "anonymous"
		viewerID = sessionID
	}

	viewCount, err := h.repo.RecordView(r.Context(), postID, viewerType, viewerID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "RecordView",
			Resource:  "view",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to record view", err, ctx, h.logger)
		return
	}

	writeViewsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"view_count": viewCount,
		},
	})
}

// GetViewCount handles GET /v1/posts/:id/views - get view count for a post.
func (h *ViewsHandler) GetViewCount(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		writeViewsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	viewCount, err := h.repo.GetViewCount(r.Context(), postID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "GetViewCount",
			Resource:  "view",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to get view count", err, ctx, h.logger)
		return
	}

	writeViewsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"view_count": viewCount,
		},
	})
}

// writeViewsJSON writes a JSON response.
func writeViewsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeViewsError writes an error JSON response.
func writeViewsError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

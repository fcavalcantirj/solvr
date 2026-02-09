// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// BookmarksRepositoryInterface defines the database operations for bookmarks.
type BookmarksRepositoryInterface interface {
	Add(ctx context.Context, userType, userID, postID string) (*models.Bookmark, error)
	Remove(ctx context.Context, userType, userID, postID string) error
	ListByUser(ctx context.Context, userType, userID string, page, perPage int) ([]models.BookmarkWithPost, int, error)
	IsBookmarked(ctx context.Context, userType, userID, postID string) (bool, error)
}

// BookmarksHandler handles bookmark-related HTTP requests.
type BookmarksHandler struct {
	repo   BookmarksRepositoryInterface
	logger *slog.Logger
}

// NewBookmarksHandler creates a new BookmarksHandler.
func NewBookmarksHandler(repo BookmarksRepositoryInterface) *BookmarksHandler {
	return &BookmarksHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger sets a custom logger for the handler.
func (h *BookmarksHandler) SetLogger(logger *slog.Logger) {
	h.logger = logger
}

// BookmarkRequest is the request body for adding a bookmark.
type BookmarkRequest struct {
	PostID string `json:"post_id"`
}

// BookmarksListResponse is the response for listing bookmarks.
type BookmarksListResponse struct {
	Data []models.BookmarkWithPost `json:"data"`
	Meta struct {
		Total   int  `json:"total"`
		Page    int  `json:"page"`
		PerPage int  `json:"per_page"`
		HasMore bool `json:"has_more"`
	} `json:"meta"`
}

// Add handles POST /v1/users/me/bookmarks - add a bookmark.
func (h *BookmarksHandler) Add(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBookmarksError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req BookmarkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBookmarksError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	if req.PostID == "" {
		writeBookmarksError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post_id is required")
		return
	}

	bookmark, err := h.repo.Add(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, req.PostID)
	if err != nil {
		if errors.Is(err, db.ErrBookmarkExists) {
			writeBookmarksError(w, http.StatusConflict, "BOOKMARK_EXISTS", "post is already bookmarked")
			return
		}
		ctx := response.LogContext{
			Operation: "Add",
			Resource:  "bookmark",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to add bookmark", err, ctx, h.logger)
		return
	}

	writeBookmarksJSON(w, http.StatusCreated, map[string]interface{}{
		"data": bookmark,
	})
}

// Remove handles DELETE /v1/users/me/bookmarks/:id - remove a bookmark.
func (h *BookmarksHandler) Remove(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBookmarksError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writeBookmarksError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	err := h.repo.Remove(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, postID)
	if err != nil {
		if errors.Is(err, db.ErrBookmarkNotFound) {
			writeBookmarksError(w, http.StatusNotFound, "NOT_FOUND", "bookmark not found")
			return
		}
		ctx := response.LogContext{
			Operation: "Remove",
			Resource:  "bookmark",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to remove bookmark", err, ctx, h.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List handles GET /v1/users/me/bookmarks - list user's bookmarks.
func (h *BookmarksHandler) List(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBookmarksError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		response.WriteValidationError(w, err.Error(), nil)
		return
	}

	bookmarks, total, err := h.repo.ListByUser(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, page, perPage)
	if err != nil {
		ctx := response.LogContext{
			Operation: "ListByUser",
			Resource:  "bookmark",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list bookmarks", err, ctx, h.logger)
		return
	}

	hasMore := (page * perPage) < total

	resp := BookmarksListResponse{}
	resp.Data = bookmarks
	resp.Meta.Total = total
	resp.Meta.Page = page
	resp.Meta.PerPage = perPage
	resp.Meta.HasMore = hasMore

	writeBookmarksJSON(w, http.StatusOK, resp)
}

// Check handles GET /v1/users/me/bookmarks/:id - check if post is bookmarked.
func (h *BookmarksHandler) Check(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeBookmarksError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writeBookmarksError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	isBookmarked, err := h.repo.IsBookmarked(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, postID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "IsBookmarked",
			Resource:  "bookmark",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to check bookmark", err, ctx, h.logger)
		return
	}

	writeBookmarksJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"bookmarked": isBookmarked,
		},
	})
}

// writeBookmarksJSON writes a JSON response.
func writeBookmarksJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeBookmarksError writes an error JSON response.
func writeBookmarksError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

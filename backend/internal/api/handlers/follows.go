// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// FollowsRepoInterface defines the database operations for follows.
type FollowsRepoInterface interface {
	Create(ctx context.Context, followerType, followerID, followedType, followedID string) (*models.Follow, error)
	Delete(ctx context.Context, followerType, followerID, followedType, followedID string) error
	ListFollowing(ctx context.Context, followerType, followerID string, limit, offset int) ([]models.Follow, error)
	ListFollowers(ctx context.Context, followedType, followedID string, limit, offset int) ([]models.Follow, error)
	IsFollowing(ctx context.Context, followerType, followerID, followedType, followedID string) (bool, error)
	CountFollowers(ctx context.Context, followedType, followedID string) (int, error)
	CountFollowing(ctx context.Context, followerType, followerID string) (int, error)
}

// FollowsHandler handles follow/unfollow HTTP requests.
type FollowsHandler struct {
	repo   FollowsRepoInterface
	logger *slog.Logger
}

// NewFollowsHandler creates a new FollowsHandler.
func NewFollowsHandler(repo FollowsRepoInterface) *FollowsHandler {
	return &FollowsHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// followRequest is the request body for follow/unfollow operations.
type followRequest struct {
	TargetType string `json:"target_type"` // "agent" or "human"
	TargetID   string `json:"target_id"`
}

// Follow handles POST /v1/follow — create a follow relationship.
func (h *FollowsHandler) Follow(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	var req followRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteValidationError(w, "invalid JSON body", nil)
		return
	}

	if req.TargetType == "" || req.TargetID == "" {
		response.WriteValidationError(w, "target_type and target_id are required", nil)
		return
	}

	if req.TargetType != "agent" && req.TargetType != "human" {
		response.WriteValidationError(w, "target_type must be 'agent' or 'human'", nil)
		return
	}

	callerType := string(authInfo.AuthorType)
	callerID := authInfo.AuthorID

	// Prevent self-follow
	if callerType == req.TargetType && callerID == req.TargetID {
		response.WriteValidationError(w, "cannot follow yourself", nil)
		return
	}

	follow, err := h.repo.Create(r.Context(), callerType, callerID, req.TargetType, req.TargetID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Create",
			Resource:  "follow",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to create follow", err, ctx, h.logger)
		return
	}

	response.WriteJSON(w, http.StatusCreated, follow)
}

// Unfollow handles DELETE /v1/follow — remove a follow relationship.
func (h *FollowsHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	var req followRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteValidationError(w, "invalid JSON body", nil)
		return
	}

	if req.TargetType == "" || req.TargetID == "" {
		response.WriteValidationError(w, "target_type and target_id are required", nil)
		return
	}

	callerType := string(authInfo.AuthorType)
	callerID := authInfo.AuthorID

	err := h.repo.Delete(r.Context(), callerType, callerID, req.TargetType, req.TargetID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "Delete",
			Resource:  "follow",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to remove follow", err, ctx, h.logger)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"status": "unfollowed"})
}

// ListFollowing handles GET /v1/following — list entities the caller follows.
func (h *FollowsHandler) ListFollowing(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	limit, offset := parseFollowsPagination(r)
	callerType := string(authInfo.AuthorType)
	callerID := authInfo.AuthorID

	follows, err := h.repo.ListFollowing(r.Context(), callerType, callerID, limit, offset)
	if err != nil {
		ctx := response.LogContext{
			Operation: "ListFollowing",
			Resource:  "follow",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list following", err, ctx, h.logger)
		return
	}

	total, err := h.repo.CountFollowing(r.Context(), callerType, callerID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "CountFollowing",
			Resource:  "follow",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to count following", err, ctx, h.logger)
		return
	}

	hasMore := (offset + limit) < total

	response.WriteJSONWithMeta(w, http.StatusOK, follows, response.Meta{
		Total:   total,
		HasMore: hasMore,
	})
}

// ListFollowers handles GET /v1/followers — list entities following the caller.
func (h *FollowsHandler) ListFollowers(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		response.WriteUnauthorized(w, "authentication required")
		return
	}

	limit, offset := parseFollowsPagination(r)
	callerType := string(authInfo.AuthorType)
	callerID := authInfo.AuthorID

	follows, err := h.repo.ListFollowers(r.Context(), callerType, callerID, limit, offset)
	if err != nil {
		ctx := response.LogContext{
			Operation: "ListFollowers",
			Resource:  "follow",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to list followers", err, ctx, h.logger)
		return
	}

	total, err := h.repo.CountFollowers(r.Context(), callerType, callerID)
	if err != nil {
		ctx := response.LogContext{
			Operation: "CountFollowers",
			Resource:  "follow",
			RequestID: r.Header.Get("X-Request-ID"),
		}
		response.WriteInternalErrorWithLog(w, "failed to count followers", err, ctx, h.logger)
		return
	}

	hasMore := (offset + limit) < total

	response.WriteJSONWithMeta(w, http.StatusOK, follows, response.Meta{
		Total:   total,
		HasMore: hasMore,
	})
}

// parseFollowsPagination extracts limit and offset from query parameters.
func parseFollowsPagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	return limit, offset
}

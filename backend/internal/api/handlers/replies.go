// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// RepliesRepositoryInterface is the database contract for the canonical Reply
// model (BART-585). One family of operations serves every contribution.
type RepliesRepositoryInterface interface {
	Create(ctx context.Context, reply *models.Reply) (*models.Reply, error)
	GetByID(ctx context.Context, id string) (*models.ReplyWithAuthor, error)
	ListByPost(ctx context.Context, opts models.ReplyListOptions) ([]models.ReplyWithAuthor, int, error)
	ListPageByPost(ctx context.Context, params models.ReplyPageParams) ([]models.ReplyWithAuthor, int, error)
	Update(ctx context.Context, id string, authorType models.AuthorType, authorID, body string) (*models.Reply, error)
	Delete(ctx context.Context, id string, authorType models.AuthorType, authorID string) error
	Vote(ctx context.Context, replyID, voterType, voterID, direction string) error
	GetUserVote(ctx context.Context, replyID, voterType, voterID string) (*string, error)
}

// RepliesHandler serves the one create/list/update/delete/vote reply API family
// under posts and replies. There is no approach/answer/response/comment choice.
type RepliesHandler struct {
	repo   RepliesRepositoryInterface
	logger *slog.Logger
}

// NewRepliesHandler creates a new RepliesHandler.
func NewRepliesHandler(repo RepliesRepositoryInterface) *RepliesHandler {
	return &RepliesHandler{
		repo:   repo,
		logger: slog.New(slog.NewJSONHandler(os.Stderr, nil)),
	}
}

// SetLogger sets a custom logger.
func (h *RepliesHandler) SetLogger(logger *slog.Logger) { h.logger = logger }

// Create handles POST /v1/posts/{id}/replies.
func (h *RepliesHandler) Create(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeRepliesError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	postID := chi.URLParam(r, "id")
	if postID == "" {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	var req models.CreateReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	reply := &models.Reply{
		PostID:        postID,
		ParentReplyID: req.ParentReplyID,
		AuthorType:    authInfo.AuthorType,
		AuthorID:      authInfo.AuthorID,
		Body:          req.Body,
	}
	created, err := h.repo.Create(r.Context(), reply)
	if err != nil {
		switch {
		case errors.Is(err, db.ErrReplyPostNotFound):
			writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		case errors.Is(err, db.ErrParentReplyInvalid):
			writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "parent reply is invalid for this post")
		default:
			h.logger.Error("create reply failed", "error", err, "postID", postID)
			writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create reply")
		}
		return
	}
	writeRepliesJSON(w, http.StatusCreated, map[string]any{"data": created})
}

// List handles GET /v1/posts/{id}/replies (public).
func (h *RepliesHandler) List(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	if postID == "" {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Opaque cursor pagination, default 50 / max 100, matching the room entries
	// surface (idx 73 step 2). A malformed cursor or limit is a 400.
	afterCreatedAt, afterID, limit, ok := parseReplyPage(w, r.URL.Query())
	if !ok {
		return
	}

	// Fetch one extra row to detect whether a following page exists.
	replies, total, err := h.repo.ListPageByPost(r.Context(), models.ReplyPageParams{
		PostID:         postID,
		AfterCreatedAt: afterCreatedAt,
		AfterID:        afterID,
		Limit:          limit + 1,
	})
	if err != nil {
		h.logger.Error("list replies failed", "error", err, "postID", postID)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list replies")
		return
	}

	meta := map[string]any{"total": total, "has_more": false}
	if len(replies) > limit {
		replies = replies[:limit]
		last := replies[len(replies)-1]
		meta["has_more"] = true
		meta["next_cursor"] = encodeReplyCursor(last.CreatedAt, last.ID)
	}

	writeRepliesJSON(w, http.StatusOK, map[string]any{
		"data": replies,
		"meta": meta,
	})
}

// Get handles GET /v1/replies/{id} (public). A reply link targets the canonical
// Reply identity.
func (h *RepliesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	reply, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrReplyNotFound) {
			writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "reply not found")
			return
		}
		h.logger.Error("get reply failed", "error", err, "id", id)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get reply")
		return
	}
	if reply != nil {
		// Hand the client the validator it can echo as an If-Match precondition
		// on a later edit (idx 73 step 5).
		w.Header().Set("ETag", replyETag(reply.UpdatedAt))
	}
	writeRepliesJSON(w, http.StatusOK, map[string]any{"data": reply})
}

// Update handles PATCH /v1/replies/{id} (author only).
func (h *RepliesHandler) Update(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeRepliesError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	id := chi.URLParam(r, "id")

	var req models.UpdateReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if err := req.Validate(); err != nil {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	// Read the reply's current version before writing so an opt-in If-Match
	// precondition can reject a stale edit (idx 73 step 5). The author-only
	// write check stays authoritative inside repo.Update below.
	existing, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrReplyNotFound) {
			writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "reply not found")
			return
		}
		h.logger.Error("get reply for update failed", "error", err, "id", id)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get reply")
		return
	}
	if existing != nil && enforceReplyIfMatch(w, r, existing.UpdatedAt) {
		return
	}

	updated, err := h.repo.Update(r.Context(), id, authInfo.AuthorType, authInfo.AuthorID, req.Body)
	if err != nil {
		h.writeMutationError(w, err, "update", id)
		return
	}
	// Echo the new validator so the client's next If-Match is current.
	w.Header().Set("ETag", replyETag(updated.UpdatedAt))
	writeRepliesJSON(w, http.StatusOK, map[string]any{"data": updated})
}

// Delete handles DELETE /v1/replies/{id} (author only).
func (h *RepliesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeRepliesError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id, authInfo.AuthorType, authInfo.AuthorID); err != nil {
		h.writeMutationError(w, err, "delete", id)
		return
	}
	writeRepliesJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"deleted": true}})
}

// Vote handles POST /v1/replies/{id}/vote (auth). Votes target the canonical
// Reply identity.
func (h *RepliesHandler) Vote(w http.ResponseWriter, r *http.Request) {
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writeRepliesError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}
	id := chi.URLParam(r, "id")

	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	if req.Direction != "up" && req.Direction != "down" {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "direction must be 'up' or 'down'")
		return
	}

	// Confirm the reply exists and enforce no self-voting.
	reply, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrReplyNotFound) {
			writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "reply not found")
			return
		}
		h.logger.Error("vote lookup failed", "error", err, "id", id)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load reply")
		return
	}
	if reply.AuthorType == authInfo.AuthorType && reply.AuthorID == authInfo.AuthorID {
		writeRepliesError(w, http.StatusForbidden, "FORBIDDEN", "cannot vote on your own content")
		return
	}

	if err := h.repo.Vote(r.Context(), id, string(authInfo.AuthorType), authInfo.AuthorID, req.Direction); err != nil {
		if errors.Is(err, models.ErrReplyNotFound) {
			writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "reply not found")
			return
		}
		h.logger.Error("vote failed", "error", err, "id", id)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to record vote")
		return
	}
	writeRepliesJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{"voted": true, "direction": req.Direction},
	})
}

// writeMutationError maps repository errors for update/delete to HTTP responses.
func (h *RepliesHandler) writeMutationError(w http.ResponseWriter, err error, op, id string) {
	switch {
	case errors.Is(err, models.ErrReplyNotFound):
		writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "reply not found")
	case errors.Is(err, db.ErrReplyForbidden):
		writeRepliesError(w, http.StatusForbidden, "FORBIDDEN", "you can only modify your own replies")
	default:
		h.logger.Error(op+" reply failed", "error", err, "id", id)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to "+op+" reply")
	}
}

func writeRepliesJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeRepliesError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{"code": code, "message": message},
	})
}

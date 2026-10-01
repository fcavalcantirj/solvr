// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/fcavalcantirj/solvr/internal/contentgate"
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
	// ListPageByAuthor pages one author's replies across the posts the viewer may read.
	ListPageByAuthor(ctx context.Context, params models.ReplyAuthorPageParams) ([]models.ReplyWithPost, int, error)
	// Update writes the new body with its embedding; a nil embedding clears the stored vector.
	// The write lands only while the reply is still at expected (its updated_at); nil writes
	// unconditionally. A reply that moved is a *models.VersionConflictError.
	Update(ctx context.Context, id string, authorType models.AuthorType, authorID, body string, embedding *string, expected *time.Time) (*models.Reply, error)
	Delete(ctx context.Context, id string, authorType models.AuthorType, authorID string) error
	Vote(ctx context.Context, replyID, voterType, voterID, direction string) error
	GetUserVote(ctx context.Context, replyID, voterType, voterID string) (*string, error)
	// PostVisibleTo applies the GET /v1/posts/{id} read rule, so a reply surface answers
	// 404 for a post the caller may not read (absent, deleted, or another family's).
	PostVisibleTo(ctx context.Context, postID, callerHuman string) (bool, error)
}

// RepliesHandler serves the one create/list/update/delete/vote reply API family
// under posts and replies. There is no approach/answer/response/comment choice.
type RepliesHandler struct {
	contentGate      *contentgate.Gate      // anti-abuse checks before the insert (nil admits all)
	contribModerator *ContributionModerator // async moderation of new contributions (nil: none)
	repo             RepliesRepositoryInterface
	embeddingService EmbeddingServiceInterface
	logger           *slog.Logger
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

// SetEmbeddingService makes create and edit embed the reply body so replies are
// semantically searchable (hybrid_search_replies). Without it no vector is stored.
func (h *RepliesHandler) SetEmbeddingService(svc EmbeddingServiceInterface) { h.embeddingService = svc }

// embedBody returns the vector literal for a reply body, or nil when no service is
// set or embedding fails; a failure is logged and never fails the write.
func (h *RepliesHandler) embedBody(ctx context.Context, body, replyID string) *string {
	if h.embeddingService == nil {
		return nil
	}
	embedCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	embedding, err := h.embeddingService.GenerateEmbedding(embedCtx, body)
	if err != nil {
		h.logger.Warn("failed to generate embedding for reply", "error", err, "replyID", replyID)
		return nil
	}
	vecStr := float32SliceToVectorString(embedding)
	return &vecStr
}

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

	if !h.postReadable(w, r, postID) {
		return
	}

	if refuseContent(w, h.contentGate.CheckContribution(r.Context(), string(authInfo.AuthorType), authInfo.AuthorID, req.Body)) {
		return
	}
	reply := &models.Reply{
		PostID:        postID,
		ParentReplyID: req.ParentReplyID,
		AuthorType:    authInfo.AuthorType,
		AuthorID:      authInfo.AuthorID,
		Body:          req.Body,
	}
	// Synchronous like posts: the reply is semantically searchable as soon as it exists.
	reply.EmbeddingStr = h.embedBody(r.Context(), req.Body, "")
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
	h.contribModerator.Moderate("reply", created.ID, req.Body, string(authInfo.AuthorType), authInfo.AuthorID)
	// Answer the same Reply the reads return, author included (the published ReplyResponse
	// requires it). The reply is stored either way: a failed read-back still answers it,
	// so the client does not retry and post it twice.
	if stored, err := h.repo.GetByID(r.Context(), created.ID); err == nil && stored != nil {
		writeRepliesJSON(w, http.StatusCreated, map[string]any{"data": stored})
		return
	} else if err != nil {
		h.logger.Warn("read back created reply failed", "error", err, "replyID", created.ID)
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
	if !h.postReadable(w, r, postID) {
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
	if reply != nil && !h.replyReadable(w, r, reply.PostID) {
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

	// Read the reply's current version before writing so the required If-Match
	// precondition can refuse a missing or stale one (idx 74 step 5). The
	// author-only write check stays authoritative inside repo.Update below.
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
	if existing == nil {
		writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "reply not found")
		return
	}
	// Refuse a non-author before paying for an embedding; repo.Update re-checks.
	if existing.AuthorType != authInfo.AuthorType || existing.AuthorID != authInfo.AuthorID {
		writeRepliesError(w, http.StatusForbidden, "FORBIDDEN", "you can only modify your own replies")
		return
	}
	expected, ok := enforceReplyIfMatch(w, r, existing.UpdatedAt)
	if !ok {
		return
	}

	embedding := h.embedBody(r.Context(), req.Body, id)
	// The write lands only at the version the precondition checked, so an
	// edit that lost a race to another writer is 412, not a lost update.
	updated, err := h.repo.Update(r.Context(), id, authInfo.AuthorType, authInfo.AuthorID, req.Body, embedding, expected)
	if answerVersionConflict(w, err, "reply", writeRepliesError) {
		return
	}
	if err != nil {
		h.writeMutationError(w, err, "update", id)
		return
	}
	// Echo the new validator so the client's next If-Match is current.
	w.Header().Set("ETag", replyETag(updated.UpdatedAt))
	// Answer the same Reply the reads return: an edit never changes the author, so the
	// author read before the write is the edited reply's.
	writeRepliesJSON(w, http.StatusOK, map[string]any{"data": models.ReplyWithAuthor{Reply: *updated, Author: existing.Author}})
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
	if !h.replyReadable(w, r, reply.PostID) {
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

// postReadable answers 404 "post not found" (the answer GET /v1/posts/{id} gives) and
// returns false when the caller may not read the post; a failed check is a 500.
func (h *RepliesHandler) postReadable(w http.ResponseWriter, r *http.Request, postID string) bool {
	return h.checkPostVisible(w, r, postID, "post not found")
}

// replyReadable is postReadable for a reply reached by its own id: the reply is hidden
// ("reply not found") whenever its post is.
func (h *RepliesHandler) replyReadable(w http.ResponseWriter, r *http.Request, postID string) bool {
	return h.checkPostVisible(w, r, postID, "reply not found")
}

func (h *RepliesHandler) checkPostVisible(w http.ResponseWriter, r *http.Request, postID, notFound string) bool {
	visible, err := h.repo.PostVisibleTo(r.Context(), postID, callerHumanID(r))
	if err != nil {
		h.logger.Error("post visibility check failed", "error", err, "postID", postID)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load post")
		return false
	}
	if !visible {
		writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", notFound)
		return false
	}
	return true
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

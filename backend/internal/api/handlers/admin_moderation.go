package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// POST /admin/posts/{id}/moderation (operator-only): the operator's decision on a post the
// automatic moderation left in pending_review, draft or rejected.
//
//	{"decision": "approve" | "reject", "note": "<why, at most 500 characters>"}
//
// approve sets the post open (published, approved: the only path that derives those states
// is PostRepository.UpdateStatus); reject sets it rejected. The decision is recorded as a
// flag by "system"/"operator" with the note, since the flags table is the moderation log. A
// post that is already open or closed is not pending: 409 NOT_PENDING. Until this route, a
// post whose Groq moderation never finished had no way out (SEO audit 2026-11-08: eleven
// posts, up to two months old, no flag).

// moderationNoteMax bounds the operator's note.
const moderationNoteMax = 500

// ModeratedPosts is what the decision needs of the post store.
type ModeratedPosts interface {
	FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error)
	UpdateStatus(ctx context.Context, postID string, status models.PostStatus) error
	CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error)
}

// PostModerationDecisionHandler serves the route.
type PostModerationDecisionHandler struct {
	posts ModeratedPosts
}

// NewPostModerationDecisionHandler creates the handler.
func NewPostModerationDecisionHandler(posts ModeratedPosts) *PostModerationDecisionHandler {
	return &PostModerationDecisionHandler{posts: posts}
}

type moderationDecisionRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// Decide handles POST /admin/posts/{id}/moderation.
func (h *PostModerationDecisionHandler) Decide(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	parsed, err := uuid.Parse(id)
	if err != nil {
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "the post id must be a UUID")
		return
	}
	var req moderationDecisionRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid request body")
		return
	}
	var status models.PostStatus
	switch strings.ToLower(strings.TrimSpace(req.Decision)) {
	case "approve":
		status = models.PostStatusOpen
	case "reject":
		status = models.PostStatusRejected
	default:
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "decision must be approve or reject")
		return
	}
	if utf8.RuneCountInString(req.Note) > moderationNoteMax {
		writeOperatorError(w, http.StatusBadRequest, "VALIDATION_ERROR", "note must have at most 500 characters")
		return
	}

	post, err := h.posts.FindByID(r.Context(), parsed.String())
	if errors.Is(err, db.ErrPostNotFound) {
		writeOperatorError(w, http.StatusNotFound, "NOT_FOUND", "no such post")
		return
	}
	if err != nil {
		slog.Error("moderation decision: read post failed", "post_id", parsed.String(), "error", err) //nolint:gosec // G706: a parsed UUID and enum values, not raw input
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to read the post")
		return
	}
	if post.Status == models.PostStatusOpen || post.Status == models.PostStatusClosed || post.Status == models.PostStatusStale {
		writeOperatorError(w, http.StatusConflict, "NOT_PENDING", fmt.Sprintf("the post is %s, not awaiting moderation", post.Status))
		return
	}

	if err := h.posts.UpdateStatus(r.Context(), parsed.String(), status); err != nil {
		slog.Error("moderation decision: update failed", "post_id", parsed.String(), "status", status, "error", err) //nolint:gosec // G706: a parsed UUID and enum values, not raw input
		writeOperatorError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update the post")
		return
	}
	decision := strings.ToLower(strings.TrimSpace(req.Decision))
	details := fmt.Sprintf("operator %s: %s -> %s", decision, post.Status, status)
	if note := strings.TrimSpace(req.Note); note != "" {
		details += ". " + note
	}
	if _, err := h.posts.CreateFlag(r.Context(), &models.Flag{
		TargetType: "post", TargetID: parsed, ReporterType: "system", ReporterID: "operator",
		Reason: "other", Details: details, Status: "actioned",
	}); err != nil {
		// The decision stands; the log entry is the part that failed.
		slog.Error("moderation decision: flag not recorded", "post_id", parsed.String(), "error", err) //nolint:gosec // G706: a parsed UUID and enum values, not raw input
	}
	slog.Info("moderation decision", "post_id", parsed.String(), "decision", status, "from", post.Status, "to", status) //nolint:gosec // G706: a parsed UUID and enum values, not raw input
	writeActivationJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"id": parsed.String(), "decision": decision, "from_status": post.Status, "status": status,
	}})
}

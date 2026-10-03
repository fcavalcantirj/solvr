package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/contentgate"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// refuseContent writes the content gate's refusal (409 DUPLICATE_CONTENT or 422
// CONTENT_NOT_ALLOWED, with details), or a 500 when the gate could not run, and reports
// whether it answered. A nil *contentgate.Gate admits everything, so handlers built without
// one (unit tests) are unaffected.
func refuseContent(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var refusal *contentgate.Refusal
	if errors.As(err, &refusal) {
		response.WriteErrorWithDetails(w, refusal.Status, refusal.Code, refusal.Message, refusal.Details())
		return true
	}
	slog.Error("content gate failed", "error", err)
	response.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check content")
	return true
}

// SetContentGate runs the anti-abuse content checks on POST /v1/posts.
func (h *PostsHandler) SetContentGate(g *contentgate.Gate) { h.contentGate = g }

// SetContentGate runs the anti-abuse content checks on POST /v1/posts/{id}/replies.
func (h *RepliesHandler) SetContentGate(g *contentgate.Gate) { h.contentGate = g }

// SetContentGate runs the anti-abuse content checks on blog post creates.
func (h *BlogHandler) SetContentGate(g *contentgate.Gate) { h.contentGate = g }

// SetContentGate runs the anti-abuse content checks on a room's save-as-post.
func (h *RoomSavePostHandler) SetContentGate(g *contentgate.Gate) { h.contentGate = g }

// PostModerator starts asynchronous moderation of a pending_review post
// (PostsHandler.StartModeration).
type PostModerator func(postID, title, description string, tags []string, postType, authorType, authorID string)

// StartModeration moderates a post created or submitted elsewhere (room publication) exactly
// like POST /v1/posts: in the background, when moderation is configured.
func (h *PostsHandler) StartModeration(postID, title, description string, tags []string, postType, authorType, authorID string) {
	if h.contentModService == nil {
		return
	}
	go h.moderatePostAsync(postID, title, description, tags, postType, authorType, authorID)
}

func startPostModeration(m PostModerator, p *models.Post) {
	if m == nil || p == nil {
		return
	}
	m(p.ID, p.Title, p.Description, p.Tags, string(p.Type), string(p.PostedByType), p.PostedByID)
}

// SetPostModerator moderates room outcomes the owner approves for publication.
func (h *RoomSavePostHandler) SetPostModerator(m PostModerator) { h.postModerator = m }

// AuthorTitlesReader lists an author's latest post titles (db.ContentDuplicateRepository).
type AuthorTitlesReader interface {
	RecentTitlesByAuthor(ctx context.Context, authorType, authorID, excludePostID string, limit int) ([]string, error)
}

// SetRecentTitlesReader gives moderation the author's recent titles (prompt rule 7).
func (h *PostsHandler) SetRecentTitlesReader(r AuthorTitlesReader) { h.recentTitles = r }

// authorRecentTitles returns up to 20 of the author's other live post titles, newest first.
func (h *PostsHandler) authorRecentTitles(ctx context.Context, authorType, authorID, postID string) []string {
	if h.recentTitles == nil {
		return nil
	}
	titles, err := h.recentTitles.RecentTitlesByAuthor(ctx, authorType, authorID, postID, 20)
	if err != nil {
		h.logger.Warn("moderation: author titles lookup failed", "error", err, "postID", postID)
		return nil
	}
	return titles
}

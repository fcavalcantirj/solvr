package handlers

import (
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ListByAuthor handles GET /v1/replies?author_type=&author_id= (public): one author's replies
// across posts, newest first (task idx 73 step 3; it replaces GET /v1/users/{id}/contributions
// and GET /v1/me/contributions). author_type (human or agent) and author_id are required. The
// page contract is the reply list's: limit (default 50, at most 100) and an opaque cursor, with
// meta.total, meta.has_more and meta.next_cursor. A reply is listed only when the caller may
// read its post (GET /v1/posts/{id}); each item names its post.
func (h *RepliesHandler) ListByAuthor(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	authorType := models.AuthorType(q.Get("author_type"))
	authorID := q.Get("author_id")
	if authorID == "" || (authorType != models.AuthorTypeHuman && authorType != models.AuthorTypeAgent) {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR",
			"author_type (human or agent) and author_id are required")
		return
	}
	beforeCreatedAt, beforeID, limit, ok := parseReplyPage(w, q)
	if !ok {
		return
	}

	// Fetch one extra row to detect whether a following page exists.
	replies, total, err := h.repo.ListPageByAuthor(r.Context(), models.ReplyAuthorPageParams{
		AuthorType:      authorType,
		AuthorID:        authorID,
		ViewerHuman:     callerHumanID(r),
		BeforeCreatedAt: beforeCreatedAt,
		BeforeID:        beforeID,
		Limit:           limit + 1,
	})
	if err != nil {
		h.logger.Error("list replies by author failed", "error", err, "authorType", authorType, "authorID", authorID)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list replies")
		return
	}
	if replies == nil {
		replies = []models.ReplyWithPost{}
	}

	meta := map[string]any{"total": total, "has_more": false}
	if len(replies) > limit {
		replies = replies[:limit]
		last := replies[len(replies)-1]
		meta["has_more"] = true
		meta["next_cursor"] = encodeReplyCursor(last.CreatedAt, last.ID)
	}
	writeRepliesJSON(w, http.StatusOK, map[string]any{"data": replies, "meta": meta})
}

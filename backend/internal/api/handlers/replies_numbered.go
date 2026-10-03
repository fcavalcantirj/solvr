package handlers

import (
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ReplyPageSize is the size of a numbered replies page (task idx 81, SPEC.md Part 27.2).
const ReplyPageSize = 100

// listNumbered answers GET /v1/posts/{id}/replies?page=N: the post's replies, oldest
// first, in numbered pages of ReplyPageSize. They are the stable segments of a long
// discussion that the web renders as /posts/{id}/replies/{n}. Page 1 of a post without
// replies is empty, not missing; a page past the last is 404.
func (h *RepliesHandler) listNumbered(w http.ResponseWriter, r *http.Request, postID string) {
	q := r.URL.Query()
	if q.Has("cursor") || q.Has("limit") {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "page cannot be combined with cursor or limit")
		return
	}
	page, ok := canonicalPageNumber(q.Get("page"))
	if !ok {
		writeRepliesError(w, http.StatusBadRequest, "VALIDATION_ERROR", "page must be a positive integer")
		return
	}
	if !h.postReadable(w, r, postID) {
		return
	}
	replies, total, err := h.repo.ListByPost(r.Context(), models.ReplyListOptions{
		PostID: postID, Page: page, PerPage: ReplyPageSize,
	})
	if err != nil {
		h.logger.Error("list reply page failed", "error", err, "postID", postID, "page", page)
		writeRepliesError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list replies")
		return
	}
	totalPages := (total + ReplyPageSize - 1) / ReplyPageSize
	if page > max(1, totalPages) {
		writeRepliesError(w, http.StatusNotFound, "NOT_FOUND", "no such replies page")
		return
	}
	writeRepliesJSON(w, http.StatusOK, map[string]any{
		"data": replies,
		"meta": map[string]any{
			"total": total, "page": page, "per_page": ReplyPageSize,
			"total_pages": totalPages, "has_more": page < totalPages,
		},
	})
}

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/go-chi/chi/v5"
)

// Delete handles DELETE /v1/posts/:id - soft delete a post.
// Per SPEC.md Part 15.1 and FIX-003: Users can delete their own content, admins can delete any.
func (h *PostsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Get existing post
	existingPost, err := h.repo.FindByIDForViewer(r.Context(), postID, "", "", callerHumanID(r)) // BART-151: owner/family can find their own private post
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "Delete"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Check permission - owner or admin can delete (works for both humans and agents)
	isOwner := existingPost.PostedByType == authInfo.AuthorType && existingPost.PostedByID == authInfo.AuthorID
	isAdmin := authInfo.Role == "admin"

	if !isOwner && !isAdmin {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "you can only delete your own posts")
		return
	}

	if err := h.repo.Delete(r.Context(), postID); err != nil {
		ctx := response.LogContext{
			Operation: "Delete",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID},
		}
		response.WriteInternalErrorWithLog(w, "failed to delete post", err, ctx, h.logger)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Vote handles POST /v1/posts/:id/vote - vote on a post.
// Per SPEC.md Part 2.9 and FIX-003: Both humans and agents can vote, but not on own content.
func (h *PostsHandler) Vote(w http.ResponseWriter, r *http.Request) {
	// Require authentication (JWT or API key)
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Parse request body
	var req VoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}

	// Validate direction
	if req.Direction != "up" && req.Direction != "down" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "direction must be 'up' or 'down'")
		return
	}

	// Get post to check it exists
	post, err := h.repo.FindByIDForViewer(r.Context(), postID, "", "", callerHumanID(r)) // BART-151: family can vote on own private post
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindByID",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "Vote"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	// Cannot vote on own content (applies to both humans and agents)
	if post.PostedByType == authInfo.AuthorType && post.PostedByID == authInfo.AuthorID {
		writePostsError(w, http.StatusForbidden, "FORBIDDEN", "cannot vote on your own content")
		return
	}

	// Record vote with the appropriate voter type
	err = h.repo.Vote(r.Context(), postID, string(authInfo.AuthorType), authInfo.AuthorID, req.Direction)
	if err != nil {
		if errors.Is(err, ErrDuplicateVote) {
			writePostsError(w, http.StatusConflict, "DUPLICATE_VOTE", "you have already voted on this post")
			return
		}
		ctx := response.LogContext{
			Operation: "Vote",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{
				"postID":    postID,
				"direction": req.Direction,
				"voterType": string(authInfo.AuthorType),
				"voterID":   authInfo.AuthorID,
			},
		}
		response.WriteInternalErrorWithLog(w, "failed to record vote", err, ctx, h.logger)
		return
	}

	// Re-fetch post to get updated vote counts
	updatedPost, fetchErr := h.repo.FindByIDForViewer(r.Context(), postID, "", "", callerHumanID(r))
	if fetchErr != nil {
		// Vote was recorded but re-fetch failed — return success with zeroed scores
		writePostsJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]interface{}{
				"vote_score": 0,
				"upvotes":    0,
				"downvotes":  0,
				"user_vote":  req.Direction,
			},
		})
		return
	}

	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"vote_score": updatedPost.VoteScore,
			"upvotes":    updatedPost.Upvotes,
			"downvotes":  updatedPost.Downvotes,
			"user_vote":  req.Direction,
		},
	})
}

// GetMyVote handles GET /v1/posts/:id/my-vote - get current user's vote on a post.
func (h *PostsHandler) GetMyVote(w http.ResponseWriter, r *http.Request) {
	// Require authentication
	authInfo := GetAuthInfo(r)
	if authInfo == nil {
		writePostsError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	postID := chi.URLParam(r, "id")
	if postID == "" {
		writePostsError(w, http.StatusBadRequest, "VALIDATION_ERROR", "post ID is required")
		return
	}

	// Match the post read contract before looking up the caller's vote. GetUserVote only
	// verifies existence, so without this gate a family-only post id leaked as {"vote":null}.
	if _, err := h.repo.FindByIDForViewer(r.Context(), postID, "", "", callerHumanID(r)); err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "FindPostForUserVote",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "GetMyVote"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, h.logger)
		return
	}

	vote, err := h.repo.GetUserVote(r.Context(), postID, string(authInfo.AuthorType), authInfo.AuthorID)
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		ctx := response.LogContext{
			Operation: "GetUserVote",
			Resource:  "post",
			RequestID: r.Header.Get("X-Request-ID"),
			Extra:     map[string]string{"postID": postID, "caller": "GetMyVote"},
		}
		response.WriteInternalErrorWithLog(w, "failed to get user vote", err, ctx, h.logger)
		return
	}

	writePostsJSON(w, http.StatusOK, map[string]interface{}{
		"data": map[string]interface{}{
			"vote": vote,
		},
	})
}

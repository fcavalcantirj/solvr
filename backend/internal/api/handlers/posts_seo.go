package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/api/response"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/seo"
	"github.com/go-chi/chi/v5"
)

// PostSEO is what a post's page tells search engines (tasks idx 80, 82, SPEC.md Part 27).
// The API decides it; the page renders it as title, robots and description metadata.
type PostSEO struct {
	// Indexable is true exactly for the posts the sitemap lists (models.Post.Indexable).
	Indexable bool `json:"indexable"`
	// Title is unique among indexable posts (seo.PostTitle).
	Title string `json:"title"`
	// Description is the visible body's excerpt (seo.PostDescription).
	Description string `json:"description"`
}

// GetSEO handles GET /v1/posts/{id}/seo: the post page's search verdict. It answers
// 404 exactly when GET /v1/posts/{id} does, for the same caller, and is served
// apart from the post read so the post contract and its consumers stay as they are.
func (h *PostsHandler) GetSEO(w http.ResponseWriter, r *http.Request) {
	postID := chi.URLParam(r, "id")
	var post *models.PostWithAuthor
	var err error
	if authInfo := GetAuthInfo(r); authInfo != nil {
		post, err = h.repo.FindByIDForViewer(r.Context(), postID, authInfo.AuthorType, authInfo.AuthorID, callerHumanID(r))
	} else {
		post, err = h.repo.FindByID(r.Context(), postID)
	}
	if err != nil {
		if errors.Is(err, db.ErrPostNotFound) {
			writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
			return
		}
		response.WriteInternalErrorWithLog(w, "failed to get post", err, response.LogContext{
			Operation: "FindByID", Resource: "post", RequestID: r.Header.Get("X-Request-ID"),
			Extra: map[string]string{"postID": postID},
		}, h.logger)
		return
	}
	if post.DeletedAt != nil {
		writePostsError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		return
	}
	twins, sameAuthor := 0, 0
	if counter, ok := h.repo.(postTitleTwins); ok {
		if twins, sameAuthor, err = counter.TitleTwins(r.Context(), postID); err != nil {
			response.WriteInternalErrorWithLog(w, "failed to get post", err, response.LogContext{
				Operation: "TitleTwins", Resource: "post", RequestID: r.Header.Get("X-Request-ID"),
				Extra: map[string]string{"postID": postID},
			}, h.logger)
			return
		}
	}
	writePostsJSON(w, http.StatusOK, map[string]PostSEO{"data": {
		Indexable:   post.Post.Indexable(),
		Title:       seo.PostTitle(post.Title, post.Author.DisplayName, post.CreatedAt, twins, sameAuthor),
		Description: seo.PostDescription(post.Title, post.Description),
	}})
}

// postTitleTwins counts a post's indexable title twins (db.PostRepository.TitleTwins).
// A repository without it (a test double) keeps every title as written.
type postTitleTwins interface {
	TitleTwins(ctx context.Context, postID string) (twins, sameAuthor int, err error)
}

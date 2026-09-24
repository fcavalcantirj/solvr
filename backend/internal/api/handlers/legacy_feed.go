package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// LenientLegacyPagination rewrites a legacy query's pagination to the old lenient shape
// the canonical GET /v1/posts parser would reject: a non-integer or < 1 page/per_page is
// dropped (defaults apply) and per_page above MaxPerPage is clamped. Shared by every legacy
// discovery adapter (task idx 71).
func LenientLegacyPagination(q url.Values) {
	if n, err := strconv.Atoi(q.Get("page")); err != nil || n < 1 {
		q.Del("page")
	}
	if n, err := strconv.Atoi(q.Get("per_page")); err != nil || n < 1 {
		q.Del("per_page")
	} else if n > MaxPerPage {
		q.Set("per_page", strconv.Itoa(MaxPerPage))
	}
}

// LegacyFeedAdapter serves the legacy feed (GET /v1/feed, /v1/feed/stuck,
// /v1/feed/unanswered — family "legacy-feed") through the canonical GET /v1/posts list:
// each route pins its canonical query, the canonical parser defines filters, ordering,
// pagination and visibility, and the rows are rendered in the legacy feed item shape.
type LegacyFeedAdapter struct {
	repo PostsRepositoryInterface
}

// NewLegacyFeedAdapter creates a LegacyFeedAdapter over the canonical posts repository.
func NewLegacyFeedAdapter(repo PostsRepositoryInterface) *LegacyFeedAdapter {
	return &LegacyFeedAdapter{repo: repo}
}

// Feed handles GET /v1/feed as GET /v1/posts?sort=newest.
func (h *LegacyFeedAdapter) Feed(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "/v1/posts?sort=newest", map[string]string{"sort": "newest"})
}

// Stuck handles GET /v1/feed/stuck as GET /v1/posts?type=problem&needs_help=true.
func (h *LegacyFeedAdapter) Stuck(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "/v1/posts?type=problem&needs_help=true",
		map[string]string{"type": "problem", "needs_help": "true", "sort": "newest"})
}

// Unanswered handles GET /v1/feed/unanswered as GET /v1/posts?type=question&has_answer=false.
func (h *LegacyFeedAdapter) Unanswered(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "/v1/posts?type=question&has_answer=false",
		map[string]string{"type": "question", "has_answer": "false", "sort": "newest"})
}

func (h *LegacyFeedAdapter) serve(w http.ResponseWriter, r *http.Request, successor string, pinned map[string]string) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Link", "<"+successor+`>; rel="successor-version"`)

	q := r.URL.Query()
	for k, v := range pinned {
		q.Set(k, v)
	}
	LenientLegacyPagination(q)
	r2 := r.Clone(r.Context())
	r2.URL.RawQuery = q.Encode()

	opts, err := parsePostListOptions(r2)
	if err != nil {
		writeFeedError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	posts, total, err := h.repo.List(r.Context(), opts)
	if err != nil {
		writeFeedError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get feed")
		return
	}

	items := make([]models.FeedItem, 0, len(posts))
	for _, p := range posts {
		items = append(items, feedItemFromPost(p))
	}
	writeFeedJSON(w, http.StatusOK, models.FeedResponse{
		Data: items,
		Meta: models.FeedMeta{
			Total:   total,
			Page:    opts.Page,
			PerPage: opts.PerPage,
			HasMore: calculateHasMore(opts.Page, opts.PerPage, total),
		},
	})
}

// feedItemFromPost renders a canonical list row in the legacy feed item shape. answer_count
// is the question's answer count (0 for other types); the snippet is the first 200 bytes of
// the description, as the legacy feed query produced it.
func feedItemFromPost(p models.PostWithAuthor) models.FeedItem {
	snippet := p.Description
	if len(snippet) > 200 {
		snippet = snippet[:200] + "..."
	}
	answers := 0
	if p.Type == models.PostTypeQuestion {
		answers = p.AnswersCount
	}
	return models.FeedItem{
		ID:      p.ID,
		Type:    string(p.Type),
		Title:   p.Title,
		Snippet: snippet,
		Tags:    p.Tags,
		Status:  string(p.Status),
		Author: models.FeedAuthor{
			Type:        string(p.Author.Type),
			ID:          p.Author.ID,
			DisplayName: p.Author.DisplayName,
			AvatarURL:   p.Author.AvatarURL,
		},
		VoteScore:     p.VoteScore,
		AnswerCount:   answers,
		ApproachCount: p.ApproachesCount,
		CommentCount:  p.CommentsCount,
		CreatedAt:     p.CreatedAt,
	}
}

// calculateHasMore determines if there are more pages.
func calculateHasMore(page, perPage, total int) bool {
	return (page * perPage) < total
}

// writeFeedJSON writes a JSON response for the legacy feed adapter.
func writeFeedJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeFeedError writes an error JSON response for the legacy feed adapter.
func writeFeedError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

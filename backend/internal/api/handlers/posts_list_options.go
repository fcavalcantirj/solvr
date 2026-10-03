package handlers

import (
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// parsePostListOptions defines the filters, ordering, pagination and visibility of the
// canonical GET /v1/posts list ONCE. Legacy discovery routes (GET /v1/problems,
// /v1/questions, /v1/ideas) are adapters that reach this same parser (task idx 71).
func parsePostListOptions(r *http.Request) (models.PostListOptions, error) {
	q := r.URL.Query()

	// FIX-029: Validate pagination parameters
	page, perPage, err := parsePaginationParams(r)
	if err != nil {
		return models.PostListOptions{}, err
	}

	opts := models.PostListOptions{
		Page:    page,
		PerPage: perPage,
	}

	// Type and status filters: every post is type "post", so a type filter selects nothing
	// more, and a retired legacy type or status is refused (idx 68).
	typeFilter, err := checkLegacyPostFilters(q.Get("type"), q.Get("status"))
	if err != nil {
		return models.PostListOptions{}, err
	}
	opts.Type = typeFilter
	if statusParam := q.Get("status"); statusParam != "" {
		opts.Status = models.PostStatus(statusParam)
	}

	// Parse tags filter
	if tagsParam := q.Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse has_answer filter ("true" / "false"; anything else is ignored)
	switch q.Get("has_answer") {
	case "true":
		hasAnswer := true
		opts.HasAnswer = &hasAnswer
	case "false":
		hasAnswer := false
		opts.HasAnswer = &hasAnswer
	}

	// Parse needs_help filter (posts with a reply migrated from a stuck approach; replaces
	// the legacy GET /v1/feed/stuck query stack)
	opts.NeedsHelp = q.Get("needs_help") == "true"

	// Parse sort parameter
	if sortParam := q.Get("sort"); sortParam != "" {
		switch sortParam {
		case "newest", "new", "votes", "top", "hot", "approaches", "answers":
			opts.Sort = sortParam
		}
	}

	// Parse timeframe filter
	if tf := q.Get("timeframe"); tf != "" {
		switch tf {
		case "today", "week", "month":
			opts.Timeframe = tf
		}
	}

	// FE-024: Parse author filter for user profile pages
	if authorType := q.Get("author_type"); authorType != "" {
		opts.AuthorType = models.AuthorType(authorType)
	}
	if authorID := q.Get("author_id"); authorID != "" {
		opts.AuthorID = authorID
	}

	// Allow authenticated users to see their own hidden posts (pending_review, rejected, draft)
	if opts.AuthorID != "" {
		if authInfo := GetAuthInfo(r); authInfo != nil {
			if authInfo.AuthorID == opts.AuthorID && string(authInfo.AuthorType) == string(opts.AuthorType) {
				opts.IncludeHidden = true
			}
		}
	}

	// Pass viewer info for user_vote lookup (works when OptionalAuthMiddleware is applied)
	if authInfo := GetAuthInfo(r); authInfo != nil {
		opts.ViewerType = authInfo.AuthorType
		opts.ViewerID = authInfo.AuthorID
	}
	// BART-151: caller's family human for visibility scoping ("" = public-only).
	opts.ViewerHuman = callerHumanID(r)

	return opts, nil
}

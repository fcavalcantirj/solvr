// Package handlers contains HTTP request handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// SearchRepositoryInterface defines the database operations for search.
type SearchRepositoryInterface interface {
	// Search performs a search with the given query and options.
	// Returns results (page), total count (post-filter), search method used
	// ("hybrid_rrf" or "fulltext_only"), the top cosine similarity across ALL matches
	// before filtering (nil when no semantic measure is available), and any error.
	// See BART-155 for the similarity/confidence contract.
	Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, string, *float64, error)
}

// SearchAnalyticsInserter defines the interface for recording search analytics.
type SearchAnalyticsInserter interface {
	Insert(ctx context.Context, sq models.SearchQuery) error
}

// DefaultSearchConfidenceThreshold is the fallback cosine-similarity bar for
// meta.confident_match when the SEARCH_CONFIDENCE_THRESHOLD env override is not wired
// (e.g. in tests). Conservative (high) to bias toward ASK. See BART-155.
const DefaultSearchConfidenceThreshold = 0.85

// SearchHandler handles search-related HTTP requests.
type SearchHandler struct {
	repo                SearchRepositoryInterface
	analyticsRepo       SearchAnalyticsInserter
	confidenceThreshold float64
}

// NewSearchHandler creates a new SearchHandler.
func NewSearchHandler(repo SearchRepositoryInterface) *SearchHandler {
	return &SearchHandler{repo: repo, confidenceThreshold: DefaultSearchConfidenceThreshold}
}

// SetAnalyticsRepo injects the analytics repository for search query tracking.
func (h *SearchHandler) SetAnalyticsRepo(repo SearchAnalyticsInserter) {
	h.analyticsRepo = repo
}

// SetConfidenceThreshold overrides the cosine-similarity bar for meta.confident_match
// and the opt-in min_similarity fallback (from SEARCH_CONFIDENCE_THRESHOLD). BART-155.
func (h *SearchHandler) SetConfidenceThreshold(threshold float64) {
	h.confidenceThreshold = threshold
}

// SearchResponse is the response structure for search results.
type SearchResponse struct {
	Data []models.SearchResultResponse `json:"data"`
	Meta SearchResponseMeta            `json:"meta"`
}

// SearchResponseMeta contains metadata about the search response.
type SearchResponseMeta struct {
	Query   string `json:"query"`
	Total   int    `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
	HasMore bool   `json:"has_more"`
	TookMs  int64  `json:"took_ms"`
	Method  string `json:"method"` // "hybrid" or "fulltext" - indicates which search method was used
	// TopSimilarity is the best cosine similarity (0–1) across ALL matches before the
	// min_similarity filter + pagination; nil when no semantic measure is available
	// (e.g. fulltext-only method). See BART-155.
	TopSimilarity *float64 `json:"top_similarity,omitempty"`
	// ConfidentMatch is the server's ASK-biased "answered?" signal: true when
	// TopSimilarity clears the confidence threshold. false → the caller should ASK.
	ConfidentMatch bool `json:"confident_match"`
	// Warnings surfaces non-fatal request issues — notably unrecognized query params
	// (which are ignored, not errored) so a wrong/typo'd name never silently no-ops.
	// Omitted entirely when there are none. See BART-155 follow-up.
	Warnings []string `json:"warnings,omitempty"`
}

// Search handles GET /v1/search - search the knowledge base.
// Query params per SPEC.md Part 5.5:
//   - q: search query (required)
//   - type: filter by post type (problem|question|idea)
//   - tags: comma-separated tags
//   - status: filter by status
//   - author: filter by author_id
//   - author_type: filter by author_type (human|agent)
//   - from_date: filter posts after this date (ISO format)
//   - to_date: filter posts before this date (ISO format)
//   - sort: relevance|newest|votes|activity (default: relevance)
//   - content_types: comma-separated content sources to search (posts,answers,approaches; default: all)
//   - page: page number (default: 1)
//   - per_page: results per page (default: 20, max: 50)
//   - min_similarity: opt-in cosine floor 0–1 (honest empty below the bar; see BART-155)
//
// Unrecognized query params are ignored but reported in meta.warnings (never a silent no-op).
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Parse and validate query parameter
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeSearchError(w, http.StatusBadRequest, "VALIDATION_ERROR", "search query 'q' is required")
		return
	}

	// Parse filters
	opts := models.SearchOptions{
		Type:       r.URL.Query().Get("type"),
		Status:     r.URL.Query().Get("status"),
		Author:     r.URL.Query().Get("author"),
		AuthorType: r.URL.Query().Get("author_type"),
		Sort:       r.URL.Query().Get("sort"),
	}

	// Parse tags (comma-separated)
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		opts.Tags = strings.Split(tagsParam, ",")
		// Trim whitespace from each tag
		for i, tag := range opts.Tags {
			opts.Tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse content_types (comma-separated: posts, answers, approaches)
	if ctParam := r.URL.Query().Get("content_types"); ctParam != "" {
		opts.ContentTypes = strings.Split(ctParam, ",")
		for i, ct := range opts.ContentTypes {
			opts.ContentTypes[i] = strings.TrimSpace(ct)
		}
	}

	// Parse date filters
	if fromDate := r.URL.Query().Get("from_date"); fromDate != "" {
		parsed, err := time.Parse("2006-01-02", fromDate)
		if err != nil {
			writeSearchError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid from_date format, use YYYY-MM-DD")
			return
		}
		opts.FromDate = parsed
	}

	if toDate := r.URL.Query().Get("to_date"); toDate != "" {
		parsed, err := time.Parse("2006-01-02", toDate)
		if err != nil {
			writeSearchError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid to_date format, use YYYY-MM-DD")
			return
		}
		opts.ToDate = parsed
	}

	// Set default sort
	if opts.Sort == "" {
		opts.Sort = "relevance"
	}

	// Parse pagination
	opts.Page = parseIntParam(r.URL.Query().Get("page"), 1)
	if opts.Page < 1 {
		opts.Page = 1
	}

	opts.PerPage = parseIntParam(r.URL.Query().Get("per_page"), 20)
	if opts.PerPage < 1 {
		opts.PerPage = 20
	}
	if opts.PerPage > 50 {
		opts.PerPage = 50 // Cap at 50 per SPEC.md
	}

	// BART-155: opt-in cosine-similarity floor (0–1). Absent = no filter (full recall);
	// invalid/out-of-range values are ignored. When set, the repo returns an honest empty
	// below the bar and drops keyword-only (unmeasurable) results.
	if ms := r.URL.Query().Get("min_similarity"); ms != "" {
		if f, err := strconv.ParseFloat(ms, 64); err == nil && f >= 0 && f <= 1 {
			opts.MinSimilarity = f
		}
	}

	// BART-155 follow-up: per-request confidence bar for meta.confident_match (0–1). Lets a
	// caller set its OWN "answered?" threshold in one call without a global default change or
	// touching SEARCH_CONFIDENCE_THRESHOLD on the server. Unlike min_similarity this does NOT
	// filter results — it only decides confident_match. Absent/invalid → server default.
	confidenceThreshold := h.confidenceThreshold
	if ct := r.URL.Query().Get("confidence_threshold"); ct != "" {
		if f, err := strconv.ParseFloat(ct, 64); err == nil && f >= 0 && f <= 1 {
			confidenceThreshold = f
		}
	}

	// BART-151: caller's family human for visibility scoping ("" = public-only).
	opts.ViewerHuman = callerHumanID(r)

	// Execute search
	results, total, method, topSimilarity, err := h.repo.Search(r.Context(), query, opts)
	if err != nil {
		writeSearchError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "search failed")
		return
	}

	// Normalize search method for frontend consumption
	// Backend uses "hybrid_rrf" and "fulltext_only" internally;
	// frontend expects "hybrid" or "fulltext"
	searchMethod := "fulltext"
	if method == "hybrid_rrf" || method == "hybrid" {
		searchMethod = "hybrid"
	}

	// Convert to response format
	responseData := make([]models.SearchResultResponse, len(results))
	for i, result := range results {
		responseData[i] = result.ToResponse()
	}

	// Calculate has_more
	hasMore := (opts.Page * opts.PerPage) < total

	// Calculate took_ms
	tookMs := time.Since(start).Milliseconds()

	// Surface unrecognized query params (ignored, not errored) so a wrong/typo'd
	// name never silently no-ops. Non-breaking: still 200 with results.
	warnings := unknownParamWarnings(r.URL.Query())

	// Build response
	response := SearchResponse{
		Data: responseData,
		Meta: SearchResponseMeta{
			Query:          query,
			Total:          total,
			Page:           opts.Page,
			PerPage:        opts.PerPage,
			HasMore:        hasMore,
			TookMs:         tookMs,
			Method:         searchMethod,
			TopSimilarity:  topSimilarity,
			ConfidentMatch: models.IsConfidentMatch(topSimilarity, confidenceThreshold),
			Warnings:       warnings,
		},
	}

	writeSearchJSON(w, http.StatusOK, response)

	// Async search analytics insert (fire-and-forget, no latency impact)
	if h.analyticsRepo != nil {
		// Extract searcher identity from auth context
		searcherType := "anonymous"
		var searcherID *string
		if claims := auth.ClaimsFromContext(r.Context()); claims != nil {
			searcherType = "human"
			searcherID = &claims.UserID
		} else if agent := auth.AgentFromContext(r.Context()); agent != nil {
			searcherType = "agent"
			searcherID = &agent.ID
		}

		// Truncate query to 500 chars (matches DB CHECK constraint)
		q := query
		if len(q) > 500 {
			q = q[:500]
		}

		sq := models.SearchQuery{
			Query:           q,
			QueryNormalized: strings.ToLower(strings.TrimSpace(q)),
			ResultsCount:    total,
			SearchMethod:    searchMethod,
			DurationMs:      int(tookMs),
			SearcherType:    searcherType,
			SearcherID:      searcherID,
			Page:            opts.Page,
			UserAgent:       r.Header.Get("User-Agent"),
			SearchedAt:      start,
		}
		if opts.Type != "" {
			sq.TypeFilter = &opts.Type
		}

		// Defensive IP extraction: RealIP middleware may strip port
		ip := r.RemoteAddr
		if host, _, err := net.SplitHostPort(ip); err == nil {
			ip = host
		}
		sq.IPAddress = ip

		go func() {
			defer func() { recover() }()
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := h.analyticsRepo.Insert(ctx, sq); err != nil {
				slog.Warn("search analytics insert failed", "error", err)
			}
		}()
	}
}

// validSearchParams is the allow-list of query params GET /search understands. Any other
// param is ignored and reported in meta.warnings. Keep in sync with the .Get() calls in Search.
var validSearchParams = map[string]struct{}{
	"q": {}, "type": {}, "tags": {}, "status": {}, "author": {}, "author_type": {},
	"from_date": {}, "to_date": {}, "sort": {}, "page": {}, "per_page": {},
	"content_types": {}, "min_similarity": {}, "confidence_threshold": {},
}

// unknownParamWarnings returns a warning for each unrecognized query-param name, with a
// "did you mean" suggestion when a close valid param exists. Params beginning with "_"
// (conventional cache-bust/internal markers) are skipped to avoid false positives. Returns
// nil when every param is recognized, so meta.warnings stays omitted. BART-155 follow-up.
func unknownParamWarnings(params map[string][]string) []string {
	var warnings []string
	for name := range params {
		if name == "" || strings.HasPrefix(name, "_") {
			continue
		}
		if _, ok := validSearchParams[name]; ok {
			continue
		}
		msg := fmt.Sprintf("unknown query parameter '%s' (ignored)", name)
		if suggestion := suggestSearchParam(name); suggestion != "" {
			msg += fmt.Sprintf(" — did you mean '%s'?", suggestion)
		}
		warnings = append(warnings, msg)
	}
	sort.Strings(warnings) // deterministic order (map iteration is random)
	return warnings
}

// suggestSearchParam returns the valid param sharing the longest common prefix (≥3 chars)
// with the unknown name, ties broken alphabetically; "" when none qualifies. Deterministically
// maps e.g. "min_score" → "min_similarity" (shared "min_s") and junk like "foobar" → "".
func suggestSearchParam(unknown string) string {
	best := ""
	bestLen := 0
	for valid := range validSearchParams {
		n := commonPrefixLen(unknown, valid)
		if n < 3 {
			continue
		}
		if n > bestLen || (n == bestLen && (best == "" || valid < best)) {
			bestLen = n
			best = valid
		}
	}
	return best
}

// commonPrefixLen returns the number of leading characters shared by a and b.
func commonPrefixLen(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// parseIntParam parses a string to int with a default value.
func parseIntParam(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return val
}

// writeSearchJSON writes a JSON response.
func writeSearchJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeSearchError writes an error JSON response.
func writeSearchError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	pgvector "github.com/pgvector/pgvector-go"
)

// QueryEmbedder generates query embeddings for hybrid search.
// Defined in db package to avoid import cycle with services package.
// The services.EmbeddingService type satisfies this interface.
type QueryEmbedder interface {
	GenerateQueryEmbedding(ctx context.Context, text string) ([]float32, error)
}

// SearchRepository implements SearchRepositoryInterface for PostgreSQL.
type SearchRepository struct {
	pool             *Pool
	embeddingService QueryEmbedder
}

// NewSearchRepository creates a new SearchRepository.
func NewSearchRepository(pool *Pool) *SearchRepository {
	return &SearchRepository{pool: pool}
}

// SetEmbeddingService sets the embedding service for hybrid search.
// When set, Search() uses hybrid RRF (full-text + vector similarity).
// When nil, Search() falls back to full-text only.
func (r *SearchRepository) SetEmbeddingService(svc QueryEmbedder) {
	r.embeddingService = svc
}

// Search performs a search across posts and their replies. With no ContentTypes it searches
// the canonical knowledge (task idx 53): posts found by their own text or by a reply's, one
// result per post, each matching reply attached as an anchor (searchKnowledge). ContentTypes
// "posts" searches the posts' own text only; "answers" and "approaches" return the replies the
// post counts put in those buckets as results of their own.
// When an embedding service is configured, uses hybrid RRF search
// (combining full-text keyword matching with vector semantic similarity).
// Falls back to full-text only search if embedding service is nil or fails.
func (r *SearchRepository) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, string, *float64, error) {
	start := time.Now()
	tsquery := buildTsQuery(query)
	if tsquery == "" {
		return []models.SearchResult{}, 0, "", nil, nil
	}

	// Try to generate query embedding for hybrid search
	var queryEmbedding []float32
	searchMethod := "fulltext_only"
	if r.embeddingService != nil {
		embStart := time.Now()
		emb, err := r.embeddingService.GenerateQueryEmbedding(ctx, query)
		if err != nil {
			// Hybrid search combines exact keyword matching (full-text) with semantic similarity (vector)
			// If embedding generation fails, fall back to full-text only search
			LogSearchEmbeddingFailed(ctx, err.Error())
		} else {
			embDuration := time.Since(embStart).Milliseconds()
			LogSearchEmbeddingGenerated(ctx, embDuration)
			queryEmbedding = emb
			searchMethod = "hybrid_rrf"
		}
	}

	contentTypes := opts.ContentTypes
	searchAll := len(contentTypes) == 0

	// A keyword search of knowledge or of posts ranks every match and builds only its page.
	if keywordPaged(queryEmbedding, opts) {
		page, total, err := r.searchKeywordPage(ctx, tsquery, opts, searchAll)
		if err != nil {
			return nil, 0, "", nil, err
		}
		LogSearchCompleted(ctx, query, time.Since(start).Milliseconds(), len(page), searchMethod)
		return page, total, searchMethod, nil, nil
	}

	var allResults []models.SearchResult

	// The default search (task idx 53) finds posts by their own text and by their replies'
	// text, one result per post; content_types=posts searches the posts' own text only.
	if searchAll {
		knowledge, err := r.searchKnowledge(ctx, queryEmbedding, tsquery, opts)
		if err != nil {
			return nil, 0, "", nil, err
		}
		allResults = append(allResults, knowledge...)
	} else if containsContentType(contentTypes, "posts") {
		posts, err := r.searchPostResults(ctx, queryEmbedding, tsquery, opts)
		if err != nil {
			return nil, 0, "", nil, err
		}
		allResults = append(allResults, posts...)
	}

	// Search reply buckets if explicitly requested (full text only)
	for _, src := range []replySearchSource{answerReplySearch, approachReplySearch} {
		if !containsContentType(contentTypes, src.contentType) {
			continue
		}
		replies, err := r.searchReplies(ctx, src, tsquery, opts)
		if err != nil {
			return nil, 0, "", nil, err
		}
		allResults = append(allResults, replies...)
	}

	// Sort merged results by score descending; ties keep the order the queries returned.
	sort.SliceStable(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// BART-155: capture the best cosine similarity across ALL matches BEFORE the opt-in
	// min_similarity floor or pagination, so meta.top_similarity / confident_match reflect
	// the true best semantic match even when the page (or the filter) yields nothing.
	topSimilarity := maxSimilarity(allResults)

	// BART-155: opt-in honest-empty filter. When MinSimilarity > 0, keep only results whose
	// cosine similarity clears the bar; drop unmeasurable (nil-similarity, keyword-only)
	// results so an unmeasured match is never presented as confident (bias to ASK). When
	// nothing clears the bar the result is a true empty (data:[], total:0) — no fuzzy fallbacks.
	if opts.MinSimilarity > 0 {
		kept := make([]models.SearchResult, 0, len(allResults))
		for _, res := range allResults {
			if res.Similarity != nil && *res.Similarity >= opts.MinSimilarity {
				kept = append(kept, res)
			}
		}
		allResults = kept
	}

	// Apply pagination
	total := len(allResults)
	limit, offset := searchPage(opts)
	if offset >= total {
		duration := time.Since(start).Milliseconds()
		LogSearchCompleted(ctx, query, duration, 0, searchMethod)
		return []models.SearchResult{}, total, searchMethod, topSimilarity, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	duration := time.Since(start).Milliseconds()
	LogSearchCompleted(ctx, query, duration, len(allResults[offset:end]), searchMethod)

	return allResults[offset:end], total, searchMethod, topSimilarity, nil
}

// searchPage is the page Search returns: per_page defaults to 20 and is capped at 50, and a page
// before the first is the first.
func searchPage(opts models.SearchOptions) (limit, offset int) {
	limit = opts.PerPage
	if limit == 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	offset = (opts.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// maxSimilarity returns a pointer to the highest non-nil Similarity across results,
// or nil when no result carries a semantic (cosine) measure. It is the input to the
// server's confident_match / top_similarity decision (BART-155).
func maxSimilarity(results []models.SearchResult) *float64 {
	var top *float64
	for i := range results {
		s := results[i].Similarity
		if s == nil {
			continue
		}
		if top == nil || *s > *top {
			v := *s
			top = &v
		}
	}
	return top
}

// searchPosts searches posts using full-text search. It matches and ranks the stored keyword
// document (posts.search_document, migration 000130) rather than parsing each post's text.
func (r *SearchRepository) searchPosts(ctx context.Context, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	baseQuery := searchPostSelect("$1", "ts_rank(p.search_document, to_tsquery('english', $1))",
		"NULL::float8", "posts p", postReplyCountsJoin) + `
		WHERE p.deleted_at IS NULL
		AND p.status NOT IN ('pending_review', 'rejected', 'draft')
		AND p.search_document @@ to_tsquery('english', $1)
	`

	args := []any{tsquery}
	argNum := 2

	// BART-151/152: family-scoped visibility (public, or the caller's own family).
	baseQuery += " AND " + searchVisibilityClause("p", opts.ViewerHuman, &args, &argNum)

	filters, args, _ := buildSearchFilters(opts, args, argNum)
	if filters != "" {
		baseQuery += " " + filters
	}

	orderBy := getSearchOrderBy(opts.Sort)
	baseQuery += " ORDER BY " + orderBy

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		LogQueryError(ctx, "Search.Posts", "posts", err)
		return nil, fmt.Errorf("search posts query failed: %w", err)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, err
	}

	// Tag all results with source "post"
	for i := range results {
		results[i].Source = "post"
	}

	return results, nil
}

// searchPostsHybrid uses the hybrid_search SQL function to combine full-text and
// vector similarity search using Reciprocal Rank Fusion (RRF).
// The hybrid_search function returns TABLE(post_id, rrf_score) with real RRF scores,
// which we JOIN with posts to get full data and format into SearchResult.
func (r *SearchRepository) searchPostsHybrid(ctx context.Context, embedding []float32, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	queryVec := pgvector.NewVector(embedding)
	matchCount := hybridMatchCount(opts)

	// Use hybrid_search SQL function which returns (post_id, rrf_score),
	// then JOIN with posts to get full data. The rrf_score is the real
	// Reciprocal Rank Fusion score — no need to re-derive from ROW_NUMBER.
	// FTS weight 2.0 > VEC weight 1.0 so keyword matches outrank semantic-only.
	// BART-155: the similarity column is the calibrated cosine similarity (0–1) of the post
	// to the query vector. $2 is the query embedding (already bound for hybrid_search); NULL
	// when the post has no embedding. Ranking still uses hs.rrf_score below.
	baseQuery := searchPostSelect("$4", "hs.rrf_score",
		"CASE WHEN p.embedding IS NOT NULL THEN 1 - (p.embedding <=> $2::vector) END",
		"hybrid_search($1, $2, $3, 2.0, 1.0, 60, $5::uuid) hs JOIN posts p ON p.id = hs.post_id",
		postPageReplyCountsJoin) + `
		WHERE p.status NOT IN ('pending_review', 'rejected', 'draft')
	`

	// BART-151: hybrid_search filters visibility inside both CTEs via viewer_human ($5,
	// NULL for anonymous/cross-family → public-only), so joined post_ids are family-scoped.
	args := []any{tsquery, queryVec, matchCount, tsquery, nullableViewer(opts.ViewerHuman)}
	argNum := 6

	// Apply filters (reuse the same filter builder, but need to adjust field references)
	filters, args, _ := buildSearchFilters(opts, args, argNum)
	if filters != "" {
		baseQuery += " " + filters
	}

	// Order by real RRF score from hybrid_search (deterministic, survives JOINs)
	baseQuery += " ORDER BY hs.rrf_score DESC"

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		// If hybrid search fails (e.g., missing function), fall back to full-text
		LogSearchEmbeddingFailed(ctx, fmt.Sprintf("hybrid_search query failed: %v", err))
		return r.searchPosts(ctx, tsquery, opts)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, err
	}

	// Tag all results with source "post"
	for i := range results {
		results[i].Source = "post"
	}

	return results, nil
}

// hybridMatchCount is how many candidates a hybrid search function returns: three pages'
// worth, at least 60, to allow for post-filtering.
func hybridMatchCount(opts models.SearchOptions) int {
	limit := opts.PerPage
	if limit == 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if limit*3 < 60 {
		return 60
	}
	return limit * 3
}

// containsContentType checks if a content type is in the list.
func containsContentType(types []string, target string) bool {
	for _, t := range types {
		if t == target {
			return true
		}
	}
	return false
}

// searchPostSelect is the SELECT list and joins every post search query shares, in the scan
// order of scanSearchResults. tsArg is the placeholder of the tsquery the snippet highlights,
// score and similarity are the SQL of those columns, and from names the posts row as p.
// counts is the reply counts join: postPageReplyCountsJoin when the query returns at most
// hybridMatchCount posts (it reads those posts' replies), postReplyCountsJoin when it returns
// every match (one aggregate of the live replies is cheaper than a lookup per post: idx 77
// slice 19 spike, a term in all 192k of 200k posts, 7.9 s with the aggregate, 16.8 s per post).
func searchPostSelect(tsArg, score, similarity, from, counts string) string {
	return `
		SELECT
			p.id,
			p.type,
			p.title,
			p.description,
			ts_headline('english', p.description, to_tsquery('english', ` + tsArg + `),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=30, MaxFragments=1') as snippet,
			p.tags,
			p.status,
			p.posted_by_type,
			p.posted_by_id,
			COALESCE(
				CASE WHEN p.posted_by_type = 'human' THEN ` + userPublicName("u") + `
					 ELSE a.display_name
				END,
				p.posted_by_id
			) as author_name,
			` + score + ` as score,
			(p.upvotes - p.downvotes) as vote_score,
			` + postReplyCountColumns + `,
			COALESCE(p.view_count, 0) as view_count,
			p.created_at,
			` + similarity + ` as similarity
		FROM ` + from + `
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id` + counts
}

// buildTsQuery converts a search query to PostgreSQL's websearch-compatible tsquery format.
func buildTsQuery(query string) string {
	// Clean up the query
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into words and join with | (OR)
	words := strings.Fields(query)
	var escaped []string
	for _, word := range words {
		// Remove special characters that might break tsquery
		word = strings.ReplaceAll(word, "'", "''")
		word = strings.ReplaceAll(word, "\\", "")
		word = strings.ReplaceAll(word, ":", "")
		word = strings.ReplaceAll(word, "(", "")
		word = strings.ReplaceAll(word, ")", "")
		word = strings.ReplaceAll(word, "&", "")
		word = strings.ReplaceAll(word, "|", "")
		word = strings.ReplaceAll(word, "!", "")
		if word != "" {
			escaped = append(escaped, word)
		}
	}

	// Join with :* for prefix matching and | for OR
	// This gives "word1:* | word2:*" which matches any of the words
	// Using OR instead of AND makes search more forgiving and finds more results
	if len(escaped) == 0 {
		return ""
	}

	for i := range escaped {
		escaped[i] = escaped[i] + ":*"
	}

	return strings.Join(escaped, " | ")
}

// buildSearchFilters builds the WHERE clause filters based on search options.
func buildSearchFilters(opts models.SearchOptions, args []any, argNum int) (string, []any, int) {
	var filters []string

	if opts.Type != "" {
		filters = append(filters, fmt.Sprintf("AND p.type = $%d", argNum))
		args = append(args, opts.Type)
		argNum++
	}

	if opts.Status != "" {
		filters = append(filters, fmt.Sprintf("AND p.status = $%d", argNum))
		args = append(args, opts.Status)
		argNum++
	}

	if len(opts.Tags) > 0 {
		filters = append(filters, fmt.Sprintf("AND p.tags && $%d", argNum))
		args = append(args, opts.Tags)
		argNum++
	}

	if opts.Author != "" {
		filters = append(filters, fmt.Sprintf("AND p.posted_by_id = $%d", argNum))
		args = append(args, opts.Author)
		argNum++
	}

	if opts.AuthorType != "" {
		filters = append(filters, fmt.Sprintf("AND p.posted_by_type = $%d", argNum))
		args = append(args, opts.AuthorType)
		argNum++
	}

	if !opts.FromDate.IsZero() {
		filters = append(filters, fmt.Sprintf("AND p.created_at >= $%d", argNum))
		args = append(args, opts.FromDate)
		argNum++
	}

	if !opts.ToDate.IsZero() {
		filters = append(filters, fmt.Sprintf("AND p.created_at <= $%d", argNum))
		args = append(args, opts.ToDate)
		argNum++
	}

	return strings.Join(filters, " "), args, argNum
}

// getSearchOrderBy returns the ORDER BY clause based on sort option. Each ends on the post id, so
// every sort is a total order: posts that tie on the sort's keys keep one order on every request,
// and pages neither repeat nor skip them.
func getSearchOrderBy(sort string) string {
	switch sort {
	case "newest":
		return "p.created_at DESC, p.id"
	case "votes":
		return "(p.upvotes - p.downvotes) DESC, p.created_at DESC, p.id"
	case "activity":
		return "p.updated_at DESC, p.created_at DESC, p.id"
	case "relevance":
		fallthrough
	default:
		return "score DESC, p.created_at DESC, p.id"
	}
}

// scanSearchResults scans rows into SearchResult slice.
func scanSearchResults(rows pgx.Rows) ([]models.SearchResult, error) {
	var results []models.SearchResult

	for rows.Next() {
		var r models.SearchResult
		err := rows.Scan(
			&r.ID,
			&r.Type,
			&r.Title,
			&r.Description,
			&r.Snippet,
			&r.Tags,
			&r.Status,
			&r.AuthorType,
			&r.AuthorID,
			&r.AuthorName,
			&r.Score,
			&r.VoteScore,
			&r.AnswersCount,
			&r.ApproachesCount,
			&r.CommentsCount,
			&r.ViewCount,
			&r.CreatedAt,
			&r.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %w", err)
	}

	return results, nil
}

// GetPool returns the underlying database pool for testing purposes.
func (r *SearchRepository) GetPool() *Pool {
	return r.pool
}

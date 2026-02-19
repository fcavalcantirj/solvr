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

// Search performs a search across posts, answers, and approaches.
// When an embedding service is configured, uses hybrid RRF search
// (combining full-text keyword matching with vector semantic similarity).
// Falls back to full-text only search if embedding service is nil or fails.
// Supports ContentTypes filter to search specific content sources.
// When ContentTypes is empty, searches only posts (backwards compatible).
func (r *SearchRepository) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, error) {
	start := time.Now()
	tsquery := buildTsQuery(query)

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

	var allResults []models.SearchResult

	// Search posts if requested or default
	if searchAll || containsContentType(contentTypes, "posts") {
		var posts []models.SearchResult
		var err error
		if queryEmbedding != nil {
			posts, err = r.searchPostsHybrid(ctx, query, queryEmbedding, tsquery, opts)
		} else {
			posts, err = r.searchPosts(ctx, tsquery, opts)
		}
		if err != nil {
			return nil, 0, err
		}
		allResults = append(allResults, posts...)
	}

	// Search answers if explicitly requested
	if containsContentType(contentTypes, "answers") {
		answers, err := r.searchAnswers(ctx, tsquery)
		if err != nil {
			return nil, 0, err
		}
		allResults = append(allResults, answers...)
	}

	// Search approaches if explicitly requested
	if containsContentType(contentTypes, "approaches") {
		approaches, err := r.searchApproaches(ctx, tsquery)
		if err != nil {
			return nil, 0, err
		}
		allResults = append(allResults, approaches...)
	}

	// Sort merged results by score descending
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// Apply pagination
	total := len(allResults)
	limit := opts.PerPage
	if limit == 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	offset := (opts.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	if offset >= total {
		duration := time.Since(start).Milliseconds()
		LogSearchCompleted(ctx, query, duration, 0, searchMethod)
		return []models.SearchResult{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	duration := time.Since(start).Milliseconds()
	LogSearchCompleted(ctx, query, duration, len(allResults[offset:end]), searchMethod)

	return allResults[offset:end], total, nil
}

// searchPosts searches posts using full-text search (existing logic).
func (r *SearchRepository) searchPosts(ctx context.Context, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	baseQuery := `
		SELECT
			p.id,
			p.type,
			p.title,
			ts_headline('english', p.description, to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=30, MaxFragments=1') as snippet,
			p.tags,
			p.status,
			p.posted_by_type,
			p.posted_by_id,
			COALESCE(
				CASE WHEN p.posted_by_type = 'human' THEN u.display_name
					 ELSE a.display_name
				END,
				p.posted_by_id
			) as author_name,
			ts_rank(to_tsvector('english', p.title || ' ' || p.description), to_tsquery('english', $1)) as score,
			(p.upvotes - p.downvotes) as vote_score,
			COALESCE((SELECT COUNT(*) FROM answers WHERE question_id = p.id AND deleted_at IS NULL), 0) as answers_count,
			p.created_at,
			CASE WHEN p.status = 'solved' THEN p.updated_at ELSE NULL END as solved_at
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		WHERE p.deleted_at IS NULL
		AND to_tsvector('english', p.title || ' ' || p.description) @@ to_tsquery('english', $1)
	`

	args := []any{tsquery}
	argNum := 2

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
// The hybrid_search function returns SETOF posts, so we query it and format
// results into SearchResult the same way searchPosts does.
func (r *SearchRepository) searchPostsHybrid(ctx context.Context, query string, embedding []float32, tsquery string, opts models.SearchOptions) ([]models.SearchResult, error) {
	queryVec := pgvector.NewVector(embedding)

	limit := opts.PerPage
	if limit == 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	// Request more results from hybrid_search to allow for post-filtering
	matchCount := limit * 3
	if matchCount < 60 {
		matchCount = 60
	}

	// Use hybrid_search SQL function, then apply the same formatting as searchPosts
	baseQuery := `
		SELECT
			p.id,
			p.type,
			p.title,
			ts_headline('english', p.description, to_tsquery('english', $4),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=30, MaxFragments=1') as snippet,
			p.tags,
			p.status,
			p.posted_by_type,
			p.posted_by_id,
			COALESCE(
				CASE WHEN p.posted_by_type = 'human' THEN u.display_name
					 ELSE a.display_name
				END,
				p.posted_by_id
			) as author_name,
			1.0 as score,
			(p.upvotes - p.downvotes) as vote_score,
			COALESCE((SELECT COUNT(*) FROM answers WHERE question_id = p.id AND deleted_at IS NULL), 0) as answers_count,
			p.created_at,
			CASE WHEN p.status = 'solved' THEN p.updated_at ELSE NULL END as solved_at
		FROM hybrid_search($1, $2, $3, 1.0, 1.0, 60) p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		WHERE 1=1
	`

	args := []any{query, queryVec, matchCount, tsquery}
	argNum := 5

	// Apply filters (reuse the same filter builder, but need to adjust field references)
	filters, args, _ := buildSearchFilters(opts, args, argNum)
	if filters != "" {
		baseQuery += " " + filters
	}

	// For hybrid search, the ordering is already handled by the SQL function (RRF score)
	// but we preserve it through the query
	baseQuery += " LIMIT " + fmt.Sprintf("%d", limit)

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

// searchAnswers searches answers using full-text search on content.
func (r *SearchRepository) searchAnswers(ctx context.Context, tsquery string) ([]models.SearchResult, error) {
	query := `
		SELECT
			a.id::text,
			'answer' as type,
			ts_headline('english', a.content, to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=30, MaxFragments=1') as title,
			ts_headline('english', a.content, to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=80, MinWords=40, MaxFragments=1') as snippet,
			COALESCE(p.tags, ARRAY[]::text[]) as tags,
			CASE WHEN a.is_accepted THEN 'accepted' ELSE '' END as status,
			a.author_type,
			a.author_id,
			COALESCE(
				CASE WHEN a.author_type = 'human' THEN u.display_name
					 ELSE ag.display_name
				END,
				a.author_id
			) as author_name,
			ts_rank(to_tsvector('english', a.content), to_tsquery('english', $1)) as score,
			(a.upvotes - a.downvotes) as vote_score,
			0 as answers_count,
			a.created_at,
			NULL::timestamptz as solved_at
		FROM answers a
		LEFT JOIN posts p ON a.question_id = p.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		WHERE a.deleted_at IS NULL
		AND to_tsvector('english', a.content) @@ to_tsquery('english', $1)
		ORDER BY score DESC
	`

	rows, err := r.pool.Query(ctx, query, tsquery)
	if err != nil {
		LogQueryError(ctx, "Search.Answers", "answers", err)
		return nil, fmt.Errorf("search answers query failed: %w", err)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, err
	}

	// Tag all results with source "answer"
	for i := range results {
		results[i].Source = "answer"
	}

	return results, nil
}

// searchApproaches searches approaches using full-text search on angle, method, outcome, solution.
func (r *SearchRepository) searchApproaches(ctx context.Context, tsquery string) ([]models.SearchResult, error) {
	query := `
		SELECT
			a.id::text,
			'approach' as type,
			ts_headline('english',
				COALESCE(a.angle, '') || ' ' || COALESCE(a.method, ''),
				to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=50, MinWords=20, MaxFragments=1') as title,
			ts_headline('english',
				COALESCE(a.angle, '') || ' ' || COALESCE(a.method, '') || ' ' ||
				COALESCE(a.outcome, '') || ' ' || COALESCE(a.solution, ''),
				to_tsquery('english', $1),
				'StartSel=<mark>, StopSel=</mark>, MaxWords=80, MinWords=40, MaxFragments=1') as snippet,
			COALESCE(p.tags, ARRAY[]::text[]) as tags,
			a.status::text,
			a.author_type,
			a.author_id,
			COALESCE(
				CASE WHEN a.author_type = 'human' THEN u.display_name
					 ELSE ag.display_name
				END,
				a.author_id
			) as author_name,
			ts_rank(to_tsvector('english',
				COALESCE(a.angle, '') || ' ' || COALESCE(a.method, '') || ' ' ||
				COALESCE(a.outcome, '') || ' ' || COALESCE(a.solution, '')),
				to_tsquery('english', $1)) as score,
			0 as vote_score,
			0 as answers_count,
			a.created_at,
			NULL::timestamptz as solved_at
		FROM approaches a
		LEFT JOIN posts p ON a.problem_id = p.id
		LEFT JOIN users u ON a.author_type = 'human' AND a.author_id = u.id::text
		LEFT JOIN agents ag ON a.author_type = 'agent' AND a.author_id = ag.id
		WHERE a.deleted_at IS NULL
		AND to_tsvector('english',
			COALESCE(a.angle, '') || ' ' || COALESCE(a.method, '') || ' ' ||
			COALESCE(a.outcome, '') || ' ' || COALESCE(a.solution, ''))
			@@ to_tsquery('english', $1)
		ORDER BY score DESC
	`

	rows, err := r.pool.Query(ctx, query, tsquery)
	if err != nil {
		LogQueryError(ctx, "Search.Approaches", "approaches", err)
		return nil, fmt.Errorf("search approaches query failed: %w", err)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, err
	}

	// Tag all results with source "approach"
	for i := range results {
		results[i].Source = "approach"
	}

	return results, nil
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

// getSearchOrderBy returns the ORDER BY clause based on sort option.
func getSearchOrderBy(sort string) string {
	switch sort {
	case "newest":
		return "p.created_at DESC"
	case "votes":
		return "(p.upvotes - p.downvotes) DESC, p.created_at DESC"
	case "activity":
		return "p.updated_at DESC"
	case "relevance":
		fallthrough
	default:
		return "score DESC, p.created_at DESC"
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
			&r.Snippet,
			&r.Tags,
			&r.Status,
			&r.AuthorType,
			&r.AuthorID,
			&r.AuthorName,
			&r.Score,
			&r.VoteScore,
			&r.AnswersCount,
			&r.CreatedAt,
			&r.SolvedAt,
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

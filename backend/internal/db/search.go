// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
)

// SearchRepository implements SearchRepositoryInterface for PostgreSQL.
type SearchRepository struct {
	pool *Pool
}

// NewSearchRepository creates a new SearchRepository.
func NewSearchRepository(pool *Pool) *SearchRepository {
	return &SearchRepository{pool: pool}
}

// Search performs a full-text search using PostgreSQL's tsvector/tsquery.
// Implements SPEC.md Part 5.5 search requirements:
// - Full-text search with ts_rank for relevance scoring
// - ts_headline for snippets with <mark> highlights
// - Filters for type, tags, status, author, author_type, from_date, to_date
// - Sorting by relevance, newest, votes, or activity
// - Pagination with page and per_page
// - Excludes deleted posts (deleted_at IS NULL)
func (r *SearchRepository) Search(ctx context.Context, query string, opts models.SearchOptions) ([]models.SearchResult, int, error) {
	// Build the tsquery from the search query
	// Convert query to websearch format (handles phrases, OR, AND, NOT)
	tsquery := buildTsQuery(query)

	// Base query with full-text search
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
			(p.upvotes - p.downvotes) as votes,
			COALESCE((SELECT COUNT(*) FROM answers WHERE question_id = p.id AND deleted_at IS NULL), 0) as answers_count,
			p.created_at,
			CASE WHEN p.status = 'solved' THEN p.updated_at ELSE NULL END as solved_at
		FROM posts p
		LEFT JOIN users u ON p.posted_by_type = 'human' AND p.posted_by_id = u.id::text
		LEFT JOIN agents a ON p.posted_by_type = 'agent' AND p.posted_by_id = a.id
		WHERE p.deleted_at IS NULL
		AND to_tsvector('english', p.title || ' ' || p.description) @@ to_tsquery('english', $1)
	`

	// Build filters
	args := []any{tsquery}
	argNum := 2

	filters, args, argNum := buildSearchFilters(opts, args, argNum)
	if filters != "" {
		baseQuery += " " + filters
	}

	// Add ordering
	orderBy := getSearchOrderBy(opts.Sort)
	baseQuery += " ORDER BY " + orderBy

	// Add pagination
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

	baseQuery += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

	// Execute main query
	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	results, err := scanSearchResults(rows)
	if err != nil {
		return nil, 0, err
	}

	// Get total count for pagination
	total, err := r.getSearchCount(ctx, tsquery, opts)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

// buildTsQuery converts a search query to PostgreSQL's websearch-compatible tsquery format.
func buildTsQuery(query string) string {
	// Clean up the query
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// Split into words and join with & (AND)
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

	// Join with :* for prefix matching and & for AND
	// This gives "word1:* & word2:*" which matches prefixes
	if len(escaped) == 0 {
		return ""
	}

	for i := range escaped {
		escaped[i] = escaped[i] + ":*"
	}

	return strings.Join(escaped, " & ")
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
			&r.Votes,
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

// getSearchCount returns the total count of matching posts.
func (r *SearchRepository) getSearchCount(ctx context.Context, tsquery string, opts models.SearchOptions) (int, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM posts p
		WHERE p.deleted_at IS NULL
		AND to_tsvector('english', p.title || ' ' || p.description) @@ to_tsquery('english', $1)
	`

	args := []any{tsquery}
	argNum := 2

	filters, args, _ := buildSearchFilters(opts, args, argNum)
	if filters != "" {
		countQuery += " " + filters
	}

	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("count query failed: %w", err)
	}

	return total, nil
}

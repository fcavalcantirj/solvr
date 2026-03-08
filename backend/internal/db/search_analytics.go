package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// SearchAnalyticsRepository handles persistence of search query analytics.
type SearchAnalyticsRepository struct {
	pool *Pool
}

// NewSearchAnalyticsRepository creates a new SearchAnalyticsRepository.
func NewSearchAnalyticsRepository(pool *Pool) *SearchAnalyticsRepository {
	return &SearchAnalyticsRepository{pool: pool}
}

// Insert stores a single search query record.
func (r *SearchAnalyticsRepository) Insert(ctx context.Context, sq models.SearchQuery) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO search_queries (
			query, query_normalized, type_filter, results_count,
			search_method, duration_ms, searcher_type, searcher_id,
			ip_address, user_agent, page, searched_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`,
		sq.Query, sq.QueryNormalized, sq.TypeFilter, sq.ResultsCount,
		sq.SearchMethod, sq.DurationMs, sq.SearcherType, sq.SearcherID,
		nilIfEmpty(sq.IPAddress), nilIfEmpty(sq.UserAgent), sq.Page, sq.SearchedAt,
	)
	if err != nil {
		return fmt.Errorf("insert search query: %w", err)
	}
	return nil
}

// GetTrending returns the most popular search queries in the last N days.
func (r *SearchAnalyticsRepository) GetTrending(ctx context.Context, days int, limit int) ([]models.TrendingSearch, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			query_normalized,
			COUNT(*) AS search_count,
			AVG(results_count)::float AS avg_results,
			AVG(duration_ms)::float AS avg_duration
		FROM search_queries
		WHERE searched_at >= NOW() - $1 * INTERVAL '1 day'
		GROUP BY query_normalized
		ORDER BY search_count DESC
		LIMIT $2
	`, days, limit)
	if err != nil {
		return nil, fmt.Errorf("get trending: %w", err)
	}
	defer rows.Close()

	var results []models.TrendingSearch
	for rows.Next() {
		var t models.TrendingSearch
		if err := rows.Scan(&t.Query, &t.Count, &t.AvgResults, &t.AvgDuration); err != nil {
			return nil, fmt.Errorf("scan trending: %w", err)
		}
		results = append(results, t)
	}
	if results == nil {
		results = []models.TrendingSearch{}
	}
	return results, rows.Err()
}

// GetZeroResults returns queries that returned 0 results, grouped by frequency.
func (r *SearchAnalyticsRepository) GetZeroResults(ctx context.Context, days int, limit int) ([]models.TrendingSearch, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.pool.Query(ctx, `
		SELECT
			query_normalized,
			COUNT(*) AS search_count,
			0::float AS avg_results,
			AVG(duration_ms)::float AS avg_duration
		FROM search_queries
		WHERE searched_at >= NOW() - $1 * INTERVAL '1 day'
		  AND results_count = 0
		GROUP BY query_normalized
		ORDER BY search_count DESC
		LIMIT $2
	`, days, limit)
	if err != nil {
		return nil, fmt.Errorf("get zero results: %w", err)
	}
	defer rows.Close()

	var results []models.TrendingSearch
	for rows.Next() {
		var t models.TrendingSearch
		if err := rows.Scan(&t.Query, &t.Count, &t.AvgResults, &t.AvgDuration); err != nil {
			return nil, fmt.Errorf("scan zero results: %w", err)
		}
		results = append(results, t)
	}
	if results == nil {
		results = []models.TrendingSearch{}
	}
	return results, rows.Err()
}

// GetSummary returns aggregate search analytics for the last N days.
func (r *SearchAnalyticsRepository) GetSummary(ctx context.Context, days int) (models.SearchAnalytics, error) {
	var summary models.SearchAnalytics

	// Core aggregates
	err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) AS total_searches,
			COUNT(DISTINCT query_normalized) AS unique_queries,
			COALESCE(AVG(duration_ms)::float, 0) AS avg_duration_ms,
			CASE WHEN COUNT(*) = 0 THEN 0
			ELSE (COUNT(*) FILTER (WHERE results_count = 0))::float / COUNT(*)::float
			END AS zero_result_rate
		FROM search_queries
		WHERE searched_at >= NOW() - $1 * INTERVAL '1 day'
	`, days).Scan(&summary.TotalSearches, &summary.UniqueQueries, &summary.AvgDurationMs, &summary.ZeroResultRate)
	if err != nil {
		return summary, fmt.Errorf("get summary aggregates: %w", err)
	}

	// By searcher type
	rows, err := r.pool.Query(ctx, `
		SELECT searcher_type, COUNT(*)
		FROM search_queries
		WHERE searched_at >= NOW() - $1 * INTERVAL '1 day'
		GROUP BY searcher_type
	`, days)
	if err != nil {
		return summary, fmt.Errorf("get summary by searcher type: %w", err)
	}
	defer rows.Close()

	summary.BySearcherType = make(map[string]int)
	for rows.Next() {
		var stype string
		var count int
		if err := rows.Scan(&stype, &count); err != nil {
			return summary, fmt.Errorf("scan searcher type: %w", err)
		}
		summary.BySearcherType[stype] = count
	}
	if err := rows.Err(); err != nil {
		return summary, err
	}

	// Top queries
	topQueries, err := r.GetTrending(ctx, days, 10)
	if err != nil {
		return summary, err
	}
	summary.TopQueries = topQueries

	// Top zero-result queries
	topZero, err := r.GetZeroResults(ctx, days, 10)
	if err != nil {
		return summary, err
	}
	summary.TopZeroResults = topZero

	return summary, nil
}

// DeleteOlderThan removes search queries older than the given cutoff for retention.
func (r *SearchAnalyticsRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM search_queries WHERE searched_at < $1", cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old search queries: %w", err)
	}
	return tag.RowsAffected(), nil
}

// nilIfEmpty returns nil if s is empty, otherwise returns a pointer to s.
// Used for INET and VARCHAR columns that should be NULL when empty.
func nilIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

package db

import (
	"context"
	"fmt"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// KnownBotSearcherIDs is a hardcoded exclusion list of automated/cron searcher_ids.
// Derived from production analytics: e48fb1b2 (449 searches, daily cron),
// agent_NaoParis (362 searches, 3 queries x 121 times).
var KnownBotSearcherIDs = []string{"e48fb1b2", "agent_NaoParis"}

// DataAnalyticsRepository handles public data analytics queries for the /v1/data/* endpoints.
type DataAnalyticsRepository struct {
	pool *Pool
}

// NewDataAnalyticsRepository creates a new DataAnalyticsRepository.
func NewDataAnalyticsRepository(pool *Pool) *DataAnalyticsRepository {
	return &DataAnalyticsRepository{pool: pool}
}

// windowToInterval maps a window string to a safe PostgreSQL interval string.
// Only "1h", "24h", and "7d" are accepted — all others return an error.
// The returned string is NEVER interpolated from user input; it comes from a whitelist.
func windowToInterval(window string) (string, error) {
	switch window {
	case "1h":
		return "1 hour", nil
	case "24h":
		return "24 hours", nil
	case "7d":
		return "7 days", nil
	default:
		return "", fmt.Errorf("invalid window %q: must be one of 1h, 24h, 7d", window)
	}
}

// buildBotExclusionClause returns a SQL fragment that excludes known bot searcher_ids.
// The list comes from KnownBotSearcherIDs (hardcoded, not user input), so string
// interpolation is safe here.
func buildBotExclusionClause() string {
	if len(KnownBotSearcherIDs) == 0 {
		return ""
	}
	quoted := make([]string, len(KnownBotSearcherIDs))
	for i, id := range KnownBotSearcherIDs {
		quoted[i] = fmt.Sprintf("'%s'", id)
	}
	return fmt.Sprintf(
		" AND (searcher_id IS NULL OR searcher_id NOT IN (%s))",
		strings.Join(quoted, ", "),
	)
}

// GetTrendingPublic returns the top trending search queries within the given time window.
// If excludeBots is true, known automated searcher_ids are filtered out.
// limit defaults to 10 if <= 0.
func (r *DataAnalyticsRepository) GetTrendingPublic(
	ctx context.Context,
	window string,
	limit int,
	excludeBots bool,
) ([]models.TrendingSearch, error) {
	interval, err := windowToInterval(window)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}

	botClause := ""
	if excludeBots {
		botClause = buildBotExclusionClause()
	}

	query := fmt.Sprintf(`
		SELECT
			query_normalized,
			COUNT(*) AS search_count,
			AVG(results_count)::float AS avg_results,
			AVG(duration_ms)::float AS avg_duration
		FROM search_queries
		WHERE searched_at >= NOW() - INTERVAL '%s'%s
		GROUP BY query_normalized
		ORDER BY search_count DESC
		LIMIT $1
	`, interval, botClause)

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("get trending public: %w", err)
	}
	defer rows.Close()

	var results []models.TrendingSearch
	for rows.Next() {
		var t models.TrendingSearch
		if err := rows.Scan(&t.Query, &t.Count, &t.AvgResults, &t.AvgDuration); err != nil {
			return nil, fmt.Errorf("scan trending public: %w", err)
		}
		results = append(results, t)
	}
	if results == nil {
		results = []models.TrendingSearch{}
	}
	return results, rows.Err()
}

// GetBreakdown returns agent/human/anonymous search counts and total statistics for the given window.
// If excludeBots is true, known automated searcher_ids are filtered out.
func (r *DataAnalyticsRepository) GetBreakdown(
	ctx context.Context,
	window string,
	excludeBots bool,
) (models.DataBreakdown, error) {
	var bd models.DataBreakdown

	interval, err := windowToInterval(window)
	if err != nil {
		return bd, err
	}

	botClause := ""
	if excludeBots {
		botClause = buildBotExclusionClause()
	}

	// Query 1: totals and zero-result rate
	totalQuery := fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			CASE WHEN COUNT(*) = 0 THEN 0
			ELSE (COUNT(*) FILTER (WHERE results_count = 0))::float / COUNT(*)::float
			END AS zero_result_rate
		FROM search_queries
		WHERE searched_at >= NOW() - INTERVAL '%s'%s
	`, interval, botClause)

	err = r.pool.QueryRow(ctx, totalQuery).Scan(&bd.TotalSearches, &bd.ZeroResultRate)
	if err != nil {
		return bd, fmt.Errorf("get breakdown totals: %w", err)
	}

	// Query 2: by searcher_type
	typeQuery := fmt.Sprintf(`
		SELECT searcher_type, COUNT(*)
		FROM search_queries
		WHERE searched_at >= NOW() - INTERVAL '%s'%s
		GROUP BY searcher_type
	`, interval, botClause)

	rows, err := r.pool.Query(ctx, typeQuery)
	if err != nil {
		return bd, fmt.Errorf("get breakdown by type: %w", err)
	}
	defer rows.Close()

	bd.BySearcherType = make(map[string]int)
	for rows.Next() {
		var stype string
		var count int
		if err := rows.Scan(&stype, &count); err != nil {
			return bd, fmt.Errorf("scan searcher type: %w", err)
		}
		bd.BySearcherType[stype] = count
	}
	if err := rows.Err(); err != nil {
		return bd, err
	}

	return bd, nil
}

// GetCategories returns search counts grouped by type_filter (category) for the given window.
// NULL type_filter values are coalesced to "unfiltered".
// If excludeBots is true, known automated searcher_ids are filtered out.
func (r *DataAnalyticsRepository) GetCategories(
	ctx context.Context,
	window string,
	excludeBots bool,
) ([]models.DataCategory, error) {
	interval, err := windowToInterval(window)
	if err != nil {
		return nil, err
	}

	botClause := ""
	if excludeBots {
		botClause = buildBotExclusionClause()
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(type_filter, 'unfiltered') AS category, COUNT(*) AS search_count
		FROM search_queries
		WHERE searched_at >= NOW() - INTERVAL '%s'%s
		GROUP BY type_filter
		ORDER BY search_count DESC
	`, interval, botClause)

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}
	defer rows.Close()

	var results []models.DataCategory
	for rows.Next() {
		var c models.DataCategory
		if err := rows.Scan(&c.Category, &c.SearchCount); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		results = append(results, c)
	}
	if results == nil {
		results = []models.DataCategory{}
	}
	return results, rows.Err()
}

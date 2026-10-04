package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ServiceCheckRepository handles persistence of service health checks.
type ServiceCheckRepository struct {
	pool *Pool
}

// NewServiceCheckRepository creates a new ServiceCheckRepository.
func NewServiceCheckRepository(pool *Pool) *ServiceCheckRepository {
	return &ServiceCheckRepository{pool: pool}
}

// Insert stores a single health check result.
func (r *ServiceCheckRepository) Insert(ctx context.Context, check models.ServiceCheck) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO service_checks (service_name, status, response_time_ms, error_message, checked_at)
		VALUES ($1, $2, $3, $4, $5)
	`, check.ServiceName, string(check.Status), check.ResponseTimeMs, check.ErrorMessage, check.CheckedAt)
	if err != nil {
		return fmt.Errorf("insert service check: %w", err)
	}
	return nil
}

// GetLatestByService returns the most recent check of each of the given services.
func (r *ServiceCheckRepository) GetLatestByService(ctx context.Context, services []string) ([]models.ServiceCheck, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (service_name)
			id, service_name, status, response_time_ms, error_message, checked_at
		FROM service_checks
		WHERE service_name = ANY($1)
		ORDER BY service_name, checked_at DESC
	`, services)
	if err != nil {
		return nil, fmt.Errorf("get latest by service: %w", err)
	}
	defer rows.Close()

	var checks []models.ServiceCheck
	for rows.Next() {
		var c models.ServiceCheck
		if err := rows.Scan(&c.ID, &c.ServiceName, &c.Status, &c.ResponseTimeMs, &c.ErrorMessage, &c.CheckedAt); err != nil {
			return nil, fmt.Errorf("scan service check: %w", err)
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

// GetDailyAggregates returns one row per calendar day with checks among the
// last N days (today included), newest first. Each day's status is the worst
// status any of the given services had that day (outage > degraded > operational).
func (r *ServiceCheckRepository) GetDailyAggregates(ctx context.Context, days int, services []string) ([]models.DailyAggregate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			TO_CHAR(checked_at::date, 'YYYY-MM-DD') AS day,
			CASE
				WHEN bool_or(status = 'outage') THEN 'outage'
				WHEN bool_or(status = 'degraded') THEN 'degraded'
				ELSE 'operational'
			END AS worst_status
		FROM service_checks
		WHERE checked_at >= (CURRENT_DATE - ($1::int - 1))::timestamptz
			AND service_name = ANY($2)
		GROUP BY checked_at::date
		ORDER BY checked_at::date DESC
	`, days, services)
	if err != nil {
		return nil, fmt.Errorf("get daily aggregates: %w", err)
	}
	defer rows.Close()

	var aggregates []models.DailyAggregate
	for rows.Next() {
		var a models.DailyAggregate
		if err := rows.Scan(&a.Date, &a.Status); err != nil {
			return nil, fmt.Errorf("scan daily aggregate: %w", err)
		}
		aggregates = append(aggregates, a)
	}
	if aggregates == nil {
		aggregates = []models.DailyAggregate{}
	}
	return aggregates, rows.Err()
}

// GetServiceStats returns, for each of the given services with checks in the
// last N days, how many checks it had, how many were operational, and the
// average response time of the checks that measured one.
func (r *ServiceCheckRepository) GetServiceStats(ctx context.Context, days int, services []string) ([]models.ServiceStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			service_name,
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'operational'),
			AVG(response_time_ms)::float,
			COUNT(response_time_ms)
		FROM service_checks
		WHERE checked_at >= NOW() - $1 * INTERVAL '1 day'
			AND service_name = ANY($2)
		GROUP BY service_name
		ORDER BY service_name
	`, days, services)
	if err != nil {
		return nil, fmt.Errorf("get service stats: %w", err)
	}
	defer rows.Close()

	var stats []models.ServiceStat
	for rows.Next() {
		var s models.ServiceStat
		if err := rows.Scan(&s.ServiceName, &s.Checks, &s.Operational, &s.AvgResponseMs, &s.ResponseSamples); err != nil {
			return nil, fmt.Errorf("scan service stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

// DeleteOlderThan removes checks older than the given cutoff for retention.
func (r *ServiceCheckRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM service_checks WHERE checked_at < $1", cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old checks: %w", err)
	}
	return tag.RowsAffected(), nil
}

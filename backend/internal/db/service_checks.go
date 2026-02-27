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

// GetLatestByService returns the most recent check for each distinct service.
func (r *ServiceCheckRepository) GetLatestByService(ctx context.Context) ([]models.ServiceCheck, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT DISTINCT ON (service_name)
			id, service_name, status, response_time_ms, error_message, checked_at
		FROM service_checks
		ORDER BY service_name, checked_at DESC
	`)
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

// GetDailyAggregates returns one row per day for the last N days.
// Each day's status is the worst status seen that day (outage > degraded > operational).
func (r *ServiceCheckRepository) GetDailyAggregates(ctx context.Context, days int) ([]models.DailyAggregate, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT
			TO_CHAR(checked_at::date, 'YYYY-MM-DD') AS day,
			CASE
				WHEN bool_or(status = 'outage') THEN 'outage'
				WHEN bool_or(status = 'degraded') THEN 'degraded'
				ELSE 'operational'
			END AS worst_status
		FROM service_checks
		WHERE checked_at >= NOW() - ($1 || ' days')::interval
		GROUP BY checked_at::date
		ORDER BY checked_at::date DESC
	`, days)
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

// GetUptimePercentage returns the percentage of checks that were "operational"
// over the last N days. Returns 0 if no data exists.
func (r *ServiceCheckRepository) GetUptimePercentage(ctx context.Context, days int) (float64, error) {
	var pct *float64
	err := r.pool.QueryRow(ctx, `
		SELECT
			CASE WHEN COUNT(*) = 0 THEN 0
			ELSE (COUNT(*) FILTER (WHERE status = 'operational'))::float / COUNT(*)::float * 100
			END
		FROM service_checks
		WHERE checked_at >= NOW() - ($1 || ' days')::interval
	`, days).Scan(&pct)
	if err != nil {
		return 0, fmt.Errorf("get uptime percentage: %w", err)
	}
	if pct == nil {
		return 0, nil
	}
	return *pct, nil
}

// GetAvgResponseTime returns the average response time in milliseconds
// over the last N days. Returns 0 if no data exists.
func (r *ServiceCheckRepository) GetAvgResponseTime(ctx context.Context, days int) (float64, error) {
	var avg *float64
	err := r.pool.QueryRow(ctx, `
		SELECT AVG(response_time_ms)::float
		FROM service_checks
		WHERE checked_at >= NOW() - ($1 || ' days')::interval
			AND response_time_ms IS NOT NULL
	`, days).Scan(&avg)
	if err != nil {
		return 0, fmt.Errorf("get avg response time: %w", err)
	}
	if avg == nil {
		return 0, nil
	}
	return *avg, nil
}

// DeleteOlderThan removes checks older than the given cutoff for retention.
func (r *ServiceCheckRepository) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, "DELETE FROM service_checks WHERE checked_at < $1", cutoff)
	if err != nil {
		return 0, fmt.Errorf("delete old checks: %w", err)
	}
	return tag.RowsAffected(), nil
}

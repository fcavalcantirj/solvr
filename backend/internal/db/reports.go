// Package db provides database access for Solvr.
package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrReportExists is returned when a user has already reported the same content.
var ErrReportExists = errors.New("you have already reported this content")

// ReportsRepository handles database operations for reports.
type ReportsRepository struct {
	pool *Pool
}

// NewReportsRepository creates a new ReportsRepository.
func NewReportsRepository(pool *Pool) *ReportsRepository {
	return &ReportsRepository{pool: pool}
}

// Create creates a new report and returns the created report.
func (r *ReportsRepository) Create(ctx context.Context, report *models.Report) (*models.Report, error) {
	query := `
		INSERT INTO reports (target_type, target_id, reporter_type, reporter_id, reason, details, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending')
		RETURNING id, created_at
	`

	err := r.pool.QueryRow(ctx, query,
		report.TargetType,
		report.TargetID,
		report.ReporterType,
		report.ReporterID,
		report.Reason,
		report.Details,
	).Scan(&report.ID, &report.CreatedAt)

	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			// Unique constraint violation
			if pgErr.Code == "23505" {
				return nil, ErrReportExists
			}
		}
		return nil, err
	}

	report.Status = models.ReportStatusPending
	return report, nil
}

// GetByID retrieves a report by ID.
func (r *ReportsRepository) GetByID(ctx context.Context, id string) (*models.Report, error) {
	query := `
		SELECT id, target_type, target_id, reporter_type, reporter_id, reason, details, status, created_at, reviewed_at, reviewed_by
		FROM reports
		WHERE id = $1
	`

	var report models.Report
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&report.ID,
		&report.TargetType,
		&report.TargetID,
		&report.ReporterType,
		&report.ReporterID,
		&report.Reason,
		&report.Details,
		&report.Status,
		&report.CreatedAt,
		&report.ReviewedAt,
		&report.ReviewedBy,
	)

	if err != nil {
		return nil, err
	}

	return &report, nil
}

// ListPending returns all pending reports for moderation.
func (r *ReportsRepository) ListPending(ctx context.Context, page, perPage int) ([]models.Report, int, error) {
	// Get total count
	countQuery := `SELECT COUNT(*) FROM reports WHERE status = 'pending'`
	var total int
	err := r.pool.QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated reports
	query := `
		SELECT id, target_type, target_id, reporter_type, reporter_id, reason, details, status, created_at
		FROM reports
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1 OFFSET $2
	`

	offset := (page - 1) * perPage
	rows, err := r.pool.Query(ctx, query, perPage, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var reports []models.Report
	for rows.Next() {
		var report models.Report
		err := rows.Scan(
			&report.ID,
			&report.TargetType,
			&report.TargetID,
			&report.ReporterType,
			&report.ReporterID,
			&report.Reason,
			&report.Details,
			&report.Status,
			&report.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, report)
	}

	return reports, total, nil
}

// HasReported checks if a user has already reported a specific target.
func (r *ReportsRepository) HasReported(ctx context.Context, targetType models.ReportTargetType, targetID, reporterType, reporterID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM reports
			WHERE target_type = $1 AND target_id = $2 AND reporter_type = $3 AND reporter_id = $4
		)
	`

	var exists bool
	err := r.pool.QueryRow(ctx, query, targetType, targetID, reporterType, reporterID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

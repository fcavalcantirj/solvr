// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// InMemoryReportsRepository is an in-memory implementation of ReportsRepositoryInterface.
// Used for testing when no database is available.
type InMemoryReportsRepository struct {
	mu      sync.RWMutex
	reports map[string]*models.Report
}

// NewInMemoryReportsRepository creates a new in-memory reports repository.
func NewInMemoryReportsRepository() *InMemoryReportsRepository {
	return &InMemoryReportsRepository{
		reports: make(map[string]*models.Report),
	}
}

// makeReportKey creates a unique key for a report based on target and reporter.
func makeReportKey(targetType models.ReportTargetType, targetID, reporterType, reporterID string) string {
	return string(targetType) + ":" + targetID + ":" + reporterType + ":" + reporterID
}

// Create creates a new report.
func (r *InMemoryReportsRepository) Create(ctx context.Context, report *models.Report) (*models.Report, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeReportKey(report.TargetType, report.TargetID, string(report.ReporterType), report.ReporterID)
	if _, exists := r.reports[key]; exists {
		return nil, db.ErrReportExists
	}

	report.ID = uuid.New().String()
	report.Status = models.ReportStatusPending
	report.CreatedAt = time.Now()

	r.reports[key] = report
	return report, nil
}

// HasReported checks if a user has already reported a specific target.
func (r *InMemoryReportsRepository) HasReported(ctx context.Context, targetType models.ReportTargetType, targetID, reporterType, reporterID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeReportKey(targetType, targetID, reporterType, reporterID)
	_, exists := r.reports[key]
	return exists, nil
}

// Ensure InMemoryReportsRepository implements the interface (compile-time check).
var _ = func() error {
	var _ interface {
		Create(ctx context.Context, report *models.Report) (*models.Report, error)
		HasReported(ctx context.Context, targetType models.ReportTargetType, targetID, reporterType, reporterID string) (bool, error)
	} = (*InMemoryReportsRepository)(nil)
	return nil
}()

package services

import (
	"context"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// DBPinger pings the database to check connectivity.
type DBPinger interface {
	Ping(ctx context.Context) error
}

// HealthCheckerService performs real health checks against the API and the database.
type HealthCheckerService struct {
	dbPinger DBPinger
}

// NewHealthCheckerService creates a new HealthCheckerService.
func NewHealthCheckerService(dbPinger DBPinger) *HealthCheckerService {
	return &HealthCheckerService{dbPinger: dbPinger}
}

// CheckService checks a named service and returns its status, response time, and any error.
func (s *HealthCheckerService) CheckService(ctx context.Context, serviceName string) (models.ServiceCheckStatus, int, error) {
	switch serviceName {
	case "api":
		return s.checkAPI()
	case "database":
		return s.checkDatabase(ctx)
	default:
		return models.ServiceStatusOutage, 0, nil
	}
}

// checkAPI returns operational if the job itself is running (self-evident health).
func (s *HealthCheckerService) checkAPI() (models.ServiceCheckStatus, int, error) {
	// If this code is executing, the API is running. Return 1ms as nominal response time.
	return models.ServiceStatusOperational, 1, nil
}

// checkDatabase pings PostgreSQL via the pool and measures response time.
func (s *HealthCheckerService) checkDatabase(ctx context.Context) (models.ServiceCheckStatus, int, error) {
	start := time.Now()
	err := s.dbPinger.Ping(ctx)
	elapsed := int(time.Since(start).Milliseconds())

	if err != nil {
		return models.ServiceStatusOutage, elapsed, err
	}

	// Consider degraded if ping takes > 500ms
	if elapsed > 500 {
		return models.ServiceStatusDegraded, elapsed, nil
	}

	return models.ServiceStatusOperational, elapsed, nil
}

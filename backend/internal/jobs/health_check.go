package jobs

import (
	"context"
	"log"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// DefaultHealthCheckInterval is how often health checks run.
const DefaultHealthCheckInterval = 5 * time.Minute

// ServiceNames lists the services to check.
var ServiceNames = []string{"api", "database", "ipfs"}

// HealthChecker performs a health check against a named service.
type HealthChecker interface {
	CheckService(ctx context.Context, serviceName string) (status models.ServiceCheckStatus, responseTimeMs int, err error)
}

// ServiceCheckWriter writes health check results.
type ServiceCheckWriter interface {
	Insert(ctx context.Context, check models.ServiceCheck) error
}

// HealthCheckJob runs periodic health checks on all services.
type HealthCheckJob struct {
	checker HealthChecker
	writer  ServiceCheckWriter
}

// NewHealthCheckJob creates a new HealthCheckJob.
func NewHealthCheckJob(checker HealthChecker, writer ServiceCheckWriter) *HealthCheckJob {
	return &HealthCheckJob{
		checker: checker,
		writer:  writer,
	}
}

// RunOnce checks all services and stores results. Returns checked and failed counts.
func (j *HealthCheckJob) RunOnce(ctx context.Context) (checked, failed int) {
	for _, name := range ServiceNames {
		status, responseTimeMs, err := j.checker.CheckService(ctx, name)

		check := models.ServiceCheck{
			ServiceName: name,
			Status:      status,
			CheckedAt:   time.Now(),
		}

		if err != nil {
			errMsg := err.Error()
			check.ErrorMessage = &errMsg
			if status == "" {
				check.Status = models.ServiceStatusOutage
			}
		}

		if responseTimeMs > 0 {
			rt := responseTimeMs
			check.ResponseTimeMs = &rt
		}

		if writeErr := j.writer.Insert(ctx, check); writeErr != nil {
			log.Printf("Health check: failed to write check for %s: %v", name, writeErr)
			failed++
			continue
		}
		checked++
	}
	return checked, failed
}

// RunScheduled runs health checks on a schedule.
// Runs immediately on start, then repeats at the given interval.
func (j *HealthCheckJob) RunScheduled(ctx context.Context, interval time.Duration) {
	checked, failed := j.RunOnce(ctx)
	if checked > 0 || failed > 0 {
		log.Printf("Health check: %d checked, %d failed", checked, failed)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Health check job stopped")
			return
		case <-ticker.C:
			checked, failed := j.RunOnce(ctx)
			if checked > 0 || failed > 0 {
				log.Printf("Health check: %d checked, %d failed", checked, failed)
			}
		}
	}
}

package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

type mockHealthChecker struct {
	results map[string]struct {
		status models.ServiceCheckStatus
		rtMs   int
		err    error
	}
}

func (m *mockHealthChecker) CheckService(ctx context.Context, name string) (models.ServiceCheckStatus, int, error) {
	r, ok := m.results[name]
	if !ok {
		return models.ServiceStatusOutage, 0, errors.New("unknown service")
	}
	return r.status, r.rtMs, r.err
}

type mockServiceCheckWriter struct {
	checks []models.ServiceCheck
	err    error
}

func (m *mockServiceCheckWriter) Insert(ctx context.Context, check models.ServiceCheck) error {
	if m.err != nil {
		return m.err
	}
	m.checks = append(m.checks, check)
	return nil
}

func TestHealthCheckJob_RunOnce_AllOperational(t *testing.T) {
	checker := &mockHealthChecker{
		results: map[string]struct {
			status models.ServiceCheckStatus
			rtMs   int
			err    error
		}{
			"api":      {models.ServiceStatusOperational, 45, nil},
			"database": {models.ServiceStatusOperational, 8, nil},
			"ipfs":     {models.ServiceStatusOperational, 65, nil},
		},
	}
	writer := &mockServiceCheckWriter{}

	job := NewHealthCheckJob(checker, writer)
	checked, failed := job.RunOnce(context.Background())

	if checked != 3 {
		t.Errorf("expected 3 checked, got %d", checked)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}
	if len(writer.checks) != 3 {
		t.Errorf("expected 3 written checks, got %d", len(writer.checks))
	}

	// Verify each check has correct status
	for _, c := range writer.checks {
		if c.Status != models.ServiceStatusOperational {
			t.Errorf("expected operational for %s, got %s", c.ServiceName, c.Status)
		}
		if c.ResponseTimeMs == nil {
			t.Errorf("expected response time for %s", c.ServiceName)
		}
	}
}

func TestHealthCheckJob_RunOnce_PartialFailure(t *testing.T) {
	checker := &mockHealthChecker{
		results: map[string]struct {
			status models.ServiceCheckStatus
			rtMs   int
			err    error
		}{
			"api":      {models.ServiceStatusOperational, 45, nil},
			"database": {models.ServiceStatusOperational, 8, nil},
			"ipfs":     {models.ServiceStatusOutage, 0, errors.New("connection refused")},
		},
	}
	writer := &mockServiceCheckWriter{}

	job := NewHealthCheckJob(checker, writer)
	checked, failed := job.RunOnce(context.Background())

	if checked != 3 {
		t.Errorf("expected 3 checked, got %d", checked)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed, got %d", failed)
	}

	// Find the IPFS check
	for _, c := range writer.checks {
		if c.ServiceName == "ipfs" {
			if c.Status != models.ServiceStatusOutage {
				t.Errorf("expected outage for ipfs, got %s", c.Status)
			}
			if c.ErrorMessage == nil {
				t.Error("expected error message for ipfs outage")
			}
		}
	}
}

func TestHealthCheckJob_RunOnce_WriterError(t *testing.T) {
	checker := &mockHealthChecker{
		results: map[string]struct {
			status models.ServiceCheckStatus
			rtMs   int
			err    error
		}{
			"api":      {models.ServiceStatusOperational, 45, nil},
			"database": {models.ServiceStatusOperational, 8, nil},
			"ipfs":     {models.ServiceStatusOperational, 65, nil},
		},
	}
	writer := &mockServiceCheckWriter{err: errors.New("db write error")}

	job := NewHealthCheckJob(checker, writer)
	checked, failed := job.RunOnce(context.Background())

	if checked != 0 {
		t.Errorf("expected 0 checked, got %d", checked)
	}
	if failed != 3 {
		t.Errorf("expected 3 failed, got %d", failed)
	}
}

func TestHealthCheckJob_RunOnce_DegradedService(t *testing.T) {
	checker := &mockHealthChecker{
		results: map[string]struct {
			status models.ServiceCheckStatus
			rtMs   int
			err    error
		}{
			"api":      {models.ServiceStatusOperational, 45, nil},
			"database": {models.ServiceStatusDegraded, 500, nil},
			"ipfs":     {models.ServiceStatusOperational, 65, nil},
		},
	}
	writer := &mockServiceCheckWriter{}

	job := NewHealthCheckJob(checker, writer)
	checked, _ := job.RunOnce(context.Background())

	if checked != 3 {
		t.Errorf("expected 3 checked, got %d", checked)
	}

	for _, c := range writer.checks {
		if c.ServiceName == "database" {
			if c.Status != models.ServiceStatusDegraded {
				t.Errorf("expected degraded for database, got %s", c.Status)
			}
		}
	}
}

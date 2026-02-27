package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func setupServiceChecksTest(t *testing.T) (*Pool, *ServiceCheckRepository) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	// Clean up test data
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")

	repo := NewServiceCheckRepository(pool)
	return pool, repo
}

func TestServiceCheckRepository_Insert(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()
	responseTime := 42
	check := models.ServiceCheck{
		ServiceName:    "test_api",
		Status:         models.ServiceStatusOperational,
		ResponseTimeMs: &responseTime,
		CheckedAt:      time.Now(),
	}

	err := repo.Insert(ctx, check)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Verify it was inserted
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM service_checks WHERE service_name = 'test_api'").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row, got %d", count)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")
}

func TestServiceCheckRepository_Insert_Outage(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()
	errMsg := "connection refused"
	check := models.ServiceCheck{
		ServiceName:  "test_database",
		Status:       models.ServiceStatusOutage,
		ErrorMessage: &errMsg,
		CheckedAt:    time.Now(),
	}

	err := repo.Insert(ctx, check)
	if err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Verify error message stored
	var storedErr *string
	err = pool.QueryRow(ctx, "SELECT error_message FROM service_checks WHERE service_name = 'test_database'").Scan(&storedErr)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if storedErr == nil || *storedErr != "connection refused" {
		t.Errorf("expected error_message 'connection refused', got %v", storedErr)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")
}

func TestServiceCheckRepository_GetLatestByService(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()

	// Insert checks at different times
	now := time.Now()
	rt1 := 10
	rt2 := 20
	rt3 := 30

	checks := []models.ServiceCheck{
		{ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt1, CheckedAt: now.Add(-10 * time.Minute)},
		{ServiceName: "test_api", Status: models.ServiceStatusDegraded, ResponseTimeMs: &rt2, CheckedAt: now},
		{ServiceName: "test_database", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt3, CheckedAt: now},
	}
	for _, c := range checks {
		if err := repo.Insert(ctx, c); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	latest, err := repo.GetLatestByService(ctx)
	if err != nil {
		t.Fatalf("GetLatestByService() error = %v", err)
	}

	// Should return only the latest check per service
	found := map[string]models.ServiceCheck{}
	for _, c := range latest {
		if c.ServiceName == "test_api" || c.ServiceName == "test_database" {
			found[c.ServiceName] = c
		}
	}

	if len(found) < 2 {
		t.Fatalf("expected at least 2 test services, got %d", len(found))
	}

	// test_api should be the latest (degraded)
	if found["test_api"].Status != models.ServiceStatusDegraded {
		t.Errorf("expected test_api latest status 'degraded', got '%s'", found["test_api"].Status)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")
}

func TestServiceCheckRepository_GetDailyAggregates(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()
	rt := 50

	// Insert checks over multiple days
	// Today: all operational
	if err := repo.Insert(ctx, models.ServiceCheck{
		ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt, CheckedAt: now,
	}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// Yesterday: one degraded
	if err := repo.Insert(ctx, models.ServiceCheck{
		ServiceName: "test_api", Status: models.ServiceStatusDegraded, ResponseTimeMs: &rt, CheckedAt: now.Add(-24 * time.Hour),
	}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	// 2 days ago: outage
	errMsg := "timeout"
	if err := repo.Insert(ctx, models.ServiceCheck{
		ServiceName: "test_api", Status: models.ServiceStatusOutage, ErrorMessage: &errMsg, CheckedAt: now.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	aggregates, err := repo.GetDailyAggregates(ctx, 30)
	if err != nil {
		t.Fatalf("GetDailyAggregates() error = %v", err)
	}

	if len(aggregates) == 0 {
		t.Fatal("expected at least 1 aggregate")
	}

	// Verify that the aggregates contain days with different statuses
	statusMap := map[string]bool{}
	for _, a := range aggregates {
		statusMap[a.Status] = true
	}

	if !statusMap["operational"] {
		t.Error("expected at least one 'operational' day")
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")
}

func TestServiceCheckRepository_GetUptimePercentage(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()
	rt := 50

	// Insert 4 checks: 3 operational, 1 outage = 75% uptime
	for i := 0; i < 3; i++ {
		if err := repo.Insert(ctx, models.ServiceCheck{
			ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rt,
			CheckedAt: now.Add(-time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}
	if err := repo.Insert(ctx, models.ServiceCheck{
		ServiceName: "test_api", Status: models.ServiceStatusOutage,
		CheckedAt: now.Add(-4 * time.Hour),
	}); err != nil {
		t.Fatalf("Insert() error = %v", err)
	}

	pct, err := repo.GetUptimePercentage(ctx, 30)
	if err != nil {
		t.Fatalf("GetUptimePercentage() error = %v", err)
	}

	// Should be 75% (3 out of 4 checks operational)
	if pct < 74.0 || pct > 76.0 {
		t.Errorf("expected uptime ~75%%, got %.2f%%", pct)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")
}

func TestServiceCheckRepository_GetAvgResponseTime(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()
	now := time.Now()

	// Insert checks with known response times: 10, 20, 30 → avg = 20
	for i, rt := range []int{10, 20, 30} {
		rtCopy := rt
		if err := repo.Insert(ctx, models.ServiceCheck{
			ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: &rtCopy,
			CheckedAt: now.Add(-time.Duration(i) * time.Hour),
		}); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}

	avg, err := repo.GetAvgResponseTime(ctx, 30)
	if err != nil {
		t.Fatalf("GetAvgResponseTime() error = %v", err)
	}

	if avg < 19.0 || avg > 21.0 {
		t.Errorf("expected avg response time ~20ms, got %.2fms", avg)
	}

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")
}

func TestServiceCheckRepository_GetUptimePercentage_Empty(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()

	// No data → should return 0 (not error)
	pct, err := repo.GetUptimePercentage(ctx, 30)
	if err != nil {
		t.Fatalf("GetUptimePercentage() error = %v", err)
	}
	if pct != 0 {
		t.Errorf("expected 0%% for no data, got %.2f%%", pct)
	}
}

func TestServiceCheckRepository_GetAvgResponseTime_Empty(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	ctx := context.Background()

	avg, err := repo.GetAvgResponseTime(ctx, 30)
	if err != nil {
		t.Fatalf("GetAvgResponseTime() error = %v", err)
	}
	if avg != 0 {
		t.Errorf("expected 0 for no data, got %.2f", avg)
	}
}

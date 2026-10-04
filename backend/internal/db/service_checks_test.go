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

// testStatusServices are the services the readers below are asked about.
// test_ipfs is written too, as an outage, and must never be counted.
var testStatusServices = []string{"test_api", "test_database"}

func insertChecks(t *testing.T, repo *ServiceCheckRepository, checks ...models.ServiceCheck) {
	t.Helper()
	for _, c := range checks {
		if err := repo.Insert(context.Background(), c); err != nil {
			t.Fatalf("Insert() error = %v", err)
		}
	}
}

// checkMs is a measured response time.
func checkMs(v int) *int { return &v }

func TestServiceCheckRepository_GetLatestByService(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")

	now := time.Now()
	insertChecks(t, repo,
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: checkMs(10), CheckedAt: now.Add(-10 * time.Minute)},
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusDegraded, ResponseTimeMs: checkMs(20), CheckedAt: now},
		models.ServiceCheck{ServiceName: "test_database", Status: models.ServiceStatusOperational, ResponseTimeMs: checkMs(30), CheckedAt: now},
		models.ServiceCheck{ServiceName: "test_ipfs", Status: models.ServiceStatusOutage, CheckedAt: now},
	)

	latest, err := repo.GetLatestByService(context.Background(), testStatusServices)
	if err != nil {
		t.Fatalf("GetLatestByService() error = %v", err)
	}

	found := map[string]models.ServiceCheck{}
	for _, c := range latest {
		found[c.ServiceName] = c
	}
	if len(found) != 2 || len(latest) != 2 {
		t.Fatalf("expected exactly test_api and test_database, got %v", found)
	}
	if found["test_api"].Status != models.ServiceStatusDegraded {
		t.Errorf("expected test_api latest status 'degraded', got '%s'", found["test_api"].Status)
	}
}

func TestServiceCheckRepository_GetDailyAggregates(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")

	now := time.Now()
	insertChecks(t, repo,
		// Today: operational. The test_ipfs outage today is not one of the services asked about.
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: checkMs(50), CheckedAt: now},
		models.ServiceCheck{ServiceName: "test_ipfs", Status: models.ServiceStatusOutage, CheckedAt: now},
		// Yesterday: degraded.
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusDegraded, ResponseTimeMs: checkMs(50), CheckedAt: now.Add(-24 * time.Hour)},
		// Two days ago: outage.
		models.ServiceCheck{ServiceName: "test_database", Status: models.ServiceStatusOutage, CheckedAt: now.Add(-48 * time.Hour)},
	)

	aggregates, err := repo.GetDailyAggregates(context.Background(), 30, testStatusServices)
	if err != nil {
		t.Fatalf("GetDailyAggregates() error = %v", err)
	}

	got := make([]string, 0, len(aggregates))
	for _, a := range aggregates {
		got = append(got, a.Status)
	}
	want := []string{"operational", "degraded", "outage"}
	if len(got) != len(want) {
		t.Fatalf("expected days %v (newest first), got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected days %v (newest first), got %v", want, got)
		}
	}
}

func TestServiceCheckRepository_GetDailyAggregates_ThirtyCalendarDays(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")

	// One check on each of 31 calendar days, the oldest inside the rolling 30x24h window.
	now := time.Now()
	for i := 0; i <= 30; i++ {
		insertChecks(t, repo, models.ServiceCheck{
			ServiceName: "test_api", Status: models.ServiceStatusOperational,
			CheckedAt: now.Add(-time.Duration(i)*24*time.Hour + time.Second),
		})
	}

	aggregates, err := repo.GetDailyAggregates(context.Background(), 30, testStatusServices)
	if err != nil {
		t.Fatalf("GetDailyAggregates() error = %v", err)
	}
	// The history is 30 calendar days, today included: one bar per day on the page.
	if len(aggregates) != 30 {
		t.Fatalf("expected 30 days, got %d", len(aggregates))
	}
}

func TestServiceCheckRepository_GetServiceStats(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()
	defer pool.Exec(context.Background(), "DELETE FROM service_checks WHERE service_name LIKE 'test_%'")

	now := time.Now()
	insertChecks(t, repo,
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: checkMs(10), CheckedAt: now},
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusOperational, ResponseTimeMs: checkMs(20), CheckedAt: now.Add(-time.Hour)},
		models.ServiceCheck{ServiceName: "test_database", Status: models.ServiceStatusOperational, CheckedAt: now},
		models.ServiceCheck{ServiceName: "test_database", Status: models.ServiceStatusOutage, CheckedAt: now.Add(-time.Hour)},
		models.ServiceCheck{ServiceName: "test_ipfs", Status: models.ServiceStatusOutage, ResponseTimeMs: checkMs(126000), CheckedAt: now},
		// Outside the 30-day window.
		models.ServiceCheck{ServiceName: "test_api", Status: models.ServiceStatusOutage, ResponseTimeMs: checkMs(9000), CheckedAt: now.Add(-31 * 24 * time.Hour)},
	)

	stats, err := repo.GetServiceStats(context.Background(), 30, testStatusServices)
	if err != nil {
		t.Fatalf("GetServiceStats() error = %v", err)
	}

	byName := map[string]models.ServiceStat{}
	for _, s := range stats {
		byName[s.ServiceName] = s
	}
	if len(stats) != 2 {
		t.Fatalf("expected stats for test_api and test_database only, got %+v", stats)
	}

	api := byName["test_api"]
	if api.Checks != 2 || api.Operational != 2 || api.ResponseSamples != 2 || api.AvgResponseMs == nil || *api.AvgResponseMs != 15 {
		t.Errorf("test_api: expected 2 checks, 2 operational, 2 samples averaging 15ms, got %+v", api)
	}
	database := byName["test_database"]
	if database.Checks != 2 || database.Operational != 1 || database.ResponseSamples != 0 || database.AvgResponseMs != nil {
		t.Errorf("test_database: expected 2 checks, 1 operational, no latency samples, got %+v", database)
	}
}

func TestServiceCheckRepository_GetServiceStats_Empty(t *testing.T) {
	pool, repo := setupServiceChecksTest(t)
	defer pool.Close()

	stats, err := repo.GetServiceStats(context.Background(), 30, testStatusServices)
	if err != nil {
		t.Fatalf("GetServiceStats() error = %v", err)
	}
	if len(stats) != 0 {
		t.Errorf("expected no stats without checks, got %+v", stats)
	}
}

package db

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func setupIncidentsTest(t *testing.T) (*Pool, *IncidentRepository) {
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
	_, _ = pool.Exec(ctx, "DELETE FROM incident_updates WHERE incident_id LIKE 'TEST-%'")
	_, _ = pool.Exec(ctx, "DELETE FROM incidents WHERE id LIKE 'TEST-%'")

	repo := NewIncidentRepository(pool)
	return pool, repo
}

func cleanupIncidents(t *testing.T, pool *Pool) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, "DELETE FROM incident_updates WHERE incident_id LIKE 'TEST-%'")
	_, _ = pool.Exec(ctx, "DELETE FROM incidents WHERE id LIKE 'TEST-%'")
}

func TestIncidentRepository_Create(t *testing.T) {
	pool, repo := setupIncidentsTest(t)
	defer pool.Close()
	defer cleanupIncidents(t, pool)

	ctx := context.Background()
	incident := models.Incident{
		ID:               "TEST-001",
		Title:            "Elevated API latency",
		Status:           models.IncidentStatusInvestigating,
		Severity:         models.IncidentSeverityMinor,
		AffectedServices: []string{"api"},
	}

	err := repo.Create(ctx, incident)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM incidents WHERE id = 'TEST-001'").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 incident, got %d", count)
	}
}

func TestIncidentRepository_Create_Duplicate(t *testing.T) {
	pool, repo := setupIncidentsTest(t)
	defer pool.Close()
	defer cleanupIncidents(t, pool)

	ctx := context.Background()
	incident := models.Incident{
		ID:       "TEST-002",
		Title:    "Database issue",
		Status:   models.IncidentStatusInvestigating,
		Severity: models.IncidentSeverityMajor,
	}

	err := repo.Create(ctx, incident)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	// Second create should fail (duplicate PK)
	err = repo.Create(ctx, incident)
	if err == nil {
		t.Error("expected error on duplicate create, got nil")
	}
}

func TestIncidentRepository_UpdateStatus(t *testing.T) {
	pool, repo := setupIncidentsTest(t)
	defer pool.Close()
	defer cleanupIncidents(t, pool)

	ctx := context.Background()
	incident := models.Incident{
		ID:       "TEST-003",
		Title:    "IPFS connectivity",
		Status:   models.IncidentStatusInvestigating,
		Severity: models.IncidentSeverityMinor,
	}

	if err := repo.Create(ctx, incident); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	err := repo.UpdateStatus(ctx, "TEST-003", string(models.IncidentStatusResolved))
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	// Verify status changed
	var status string
	err = pool.QueryRow(ctx, "SELECT status FROM incidents WHERE id = 'TEST-003'").Scan(&status)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if status != "resolved" {
		t.Errorf("expected status 'resolved', got '%s'", status)
	}

	// Verify resolved_at was set
	var resolvedAt *string
	err = pool.QueryRow(ctx, "SELECT resolved_at::text FROM incidents WHERE id = 'TEST-003'").Scan(&resolvedAt)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resolvedAt == nil {
		t.Error("expected resolved_at to be set")
	}
}

func TestIncidentRepository_AddUpdate(t *testing.T) {
	pool, repo := setupIncidentsTest(t)
	defer pool.Close()
	defer cleanupIncidents(t, pool)

	ctx := context.Background()
	incident := models.Incident{
		ID:       "TEST-004",
		Title:    "API slowdown",
		Status:   models.IncidentStatusInvestigating,
		Severity: models.IncidentSeverityMinor,
	}

	if err := repo.Create(ctx, incident); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	update := models.IncidentUpdate{
		IncidentID: "TEST-004",
		Status:     models.IncidentStatusIdentified,
		Message:    "Root cause identified: misconfigured load balancer",
	}

	err := repo.AddUpdate(ctx, update)
	if err != nil {
		t.Fatalf("AddUpdate() error = %v", err)
	}

	// Verify update stored
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM incident_updates WHERE incident_id = 'TEST-004'").Scan(&count)
	if err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 update, got %d", count)
	}
}

func TestIncidentRepository_ListRecent(t *testing.T) {
	pool, repo := setupIncidentsTest(t)
	defer pool.Close()
	defer cleanupIncidents(t, pool)

	ctx := context.Background()

	// Create 2 incidents with updates
	inc1 := models.Incident{
		ID:       "TEST-005",
		Title:    "First incident",
		Status:   models.IncidentStatusResolved,
		Severity: models.IncidentSeverityMinor,
	}
	inc2 := models.Incident{
		ID:       "TEST-006",
		Title:    "Second incident",
		Status:   models.IncidentStatusInvestigating,
		Severity: models.IncidentSeverityMajor,
	}

	if err := repo.Create(ctx, inc1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.Create(ctx, inc2); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Add updates to first incident
	if err := repo.AddUpdate(ctx, models.IncidentUpdate{
		IncidentID: "TEST-005",
		Status:     models.IncidentStatusInvestigating,
		Message:    "Looking into it",
	}); err != nil {
		t.Fatalf("AddUpdate() error = %v", err)
	}
	if err := repo.AddUpdate(ctx, models.IncidentUpdate{
		IncidentID: "TEST-005",
		Status:     models.IncidentStatusResolved,
		Message:    "Fixed",
	}); err != nil {
		t.Fatalf("AddUpdate() error = %v", err)
	}

	incidents, err := repo.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	// Should find our test incidents
	testIncidents := []models.IncidentWithUpdates{}
	for _, inc := range incidents {
		if inc.ID == "TEST-005" || inc.ID == "TEST-006" {
			testIncidents = append(testIncidents, inc)
		}
	}

	if len(testIncidents) != 2 {
		t.Fatalf("expected 2 test incidents, got %d", len(testIncidents))
	}

	// Newer incident (TEST-006) should come first
	if testIncidents[0].ID != "TEST-006" {
		t.Errorf("expected TEST-006 first (newest), got %s", testIncidents[0].ID)
	}

	// TEST-005 should have 2 updates
	for _, inc := range testIncidents {
		if inc.ID == "TEST-005" {
			if len(inc.Updates) != 2 {
				t.Errorf("expected 2 updates for TEST-005, got %d", len(inc.Updates))
			}
		}
	}
}

func TestIncidentRepository_ListRecent_Empty(t *testing.T) {
	pool, repo := setupIncidentsTest(t)
	defer pool.Close()
	defer cleanupIncidents(t, pool)

	ctx := context.Background()

	incidents, err := repo.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	// Should return empty slice, not nil
	if incidents == nil {
		t.Error("expected empty slice, got nil")
	}
}

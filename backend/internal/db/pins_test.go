package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func setupPinTestDB(t *testing.T) *Pool {
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

	// Clean up test pin data
	_, err = pool.Exec(ctx, "DELETE FROM pins WHERE owner_id LIKE 'test-pin-%'")
	if err != nil {
		t.Logf("cleanup warning: %v", err)
	}

	return pool
}

func TestPinRepository_Create(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	pin := &models.Pin{
		CID:       "QmTest" + suffix,
		Status:    models.PinStatusQueued,
		Name:      "test-pin",
		Origins:   []string{"/ip4/127.0.0.1/tcp/4001"},
		Meta:      map[string]string{"app": "solvr-test"},
		Delegates: []string{"/ip4/10.0.0.1/tcp/4001"},
		OwnerID:   "test-pin-user-" + suffix,
		OwnerType: "user",
	}

	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify fields populated by DB
	if pin.ID == "" {
		t.Error("expected ID to be set after Create")
	}
	if pin.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if pin.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
	if pin.Status != models.PinStatusQueued {
		t.Errorf("expected status %q, got %q", models.PinStatusQueued, pin.Status)
	}
	if pin.CID != "QmTest"+suffix {
		t.Errorf("expected CID %q, got %q", "QmTest"+suffix, pin.CID)
	}
	if pin.Name != "test-pin" {
		t.Errorf("expected name %q, got %q", "test-pin", pin.Name)
	}
}

func TestPinRepository_Create_Duplicate(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	pin := &models.Pin{
		CID:       "QmDup" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   "test-pin-dup-" + suffix,
		OwnerType: "user",
	}

	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	// Try to create duplicate (same CID + same owner)
	dup := &models.Pin{
		CID:       "QmDup" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   "test-pin-dup-" + suffix,
		OwnerType: "user",
	}

	err = repo.Create(ctx, dup)
	if err != ErrDuplicatePin {
		t.Errorf("expected ErrDuplicatePin, got %v", err)
	}
}

func TestPinRepository_Create_DifferentOwnersSameCID(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	cid := "QmShared" + suffix

	// First user pins the CID
	pin1 := &models.Pin{
		CID:       cid,
		Status:    models.PinStatusQueued,
		OwnerID:   "test-pin-owner1-" + suffix,
		OwnerType: "user",
	}
	err := repo.Create(ctx, pin1)
	if err != nil {
		t.Fatalf("first Create() error = %v", err)
	}

	// Second user pins the same CID — should succeed
	pin2 := &models.Pin{
		CID:       cid,
		Status:    models.PinStatusQueued,
		OwnerID:   "test-pin-owner2-" + suffix,
		OwnerType: "agent",
	}
	err = repo.Create(ctx, pin2)
	if err != nil {
		t.Fatalf("second Create() should succeed for different owner, got error = %v", err)
	}

	if pin1.ID == pin2.ID {
		t.Error("expected different IDs for different owners")
	}
}

func TestPinRepository_GetByID(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	// Create a pin first
	pin := &models.Pin{
		CID:       "QmGetByID" + suffix,
		Status:    models.PinStatusQueued,
		Name:      "get-by-id-test",
		Origins:   []string{"/ip4/127.0.0.1/tcp/4001"},
		Meta:      map[string]string{"key": "value"},
		Delegates: []string{"/ip4/10.0.0.1/tcp/4001"},
		OwnerID:   "test-pin-getbyid-" + suffix,
		OwnerType: "user",
	}
	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Retrieve by ID
	found, err := repo.GetByID(ctx, pin.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.ID != pin.ID {
		t.Errorf("expected ID %q, got %q", pin.ID, found.ID)
	}
	if found.CID != pin.CID {
		t.Errorf("expected CID %q, got %q", pin.CID, found.CID)
	}
	if found.Name != "get-by-id-test" {
		t.Errorf("expected name %q, got %q", "get-by-id-test", found.Name)
	}
	if found.OwnerID != pin.OwnerID {
		t.Errorf("expected OwnerID %q, got %q", pin.OwnerID, found.OwnerID)
	}
	if found.Status != models.PinStatusQueued {
		t.Errorf("expected status %q, got %q", models.PinStatusQueued, found.Status)
	}
	if len(found.Origins) != 1 || found.Origins[0] != "/ip4/127.0.0.1/tcp/4001" {
		t.Errorf("expected origins [/ip4/127.0.0.1/tcp/4001], got %v", found.Origins)
	}
	if found.Meta["key"] != "value" {
		t.Errorf("expected meta key=value, got %v", found.Meta)
	}
}

func TestPinRepository_GetByID_NotFound(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
	if err != ErrPinNotFound {
		t.Errorf("expected ErrPinNotFound, got %v", err)
	}
}

func TestPinRepository_GetByCID(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	ownerID := "test-pin-getbycid-" + suffix

	pin := &models.Pin{
		CID:       "QmGetByCID" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   ownerID,
		OwnerType: "user",
	}
	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	found, err := repo.GetByCID(ctx, pin.CID, ownerID)
	if err != nil {
		t.Fatalf("GetByCID() error = %v", err)
	}

	if found.ID != pin.ID {
		t.Errorf("expected ID %q, got %q", pin.ID, found.ID)
	}
	if found.CID != pin.CID {
		t.Errorf("expected CID %q, got %q", pin.CID, found.CID)
	}
}

func TestPinRepository_GetByCID_NotFound(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()

	_, err := repo.GetByCID(ctx, "QmNonexistent", "nonexistent-owner")
	if err != ErrPinNotFound {
		t.Errorf("expected ErrPinNotFound, got %v", err)
	}
}

func TestPinRepository_ListByOwner(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	ownerID := "test-pin-list-" + suffix

	// Create 3 pins for same owner
	for i := 0; i < 3; i++ {
		pin := &models.Pin{
			CID:       "QmList" + suffix + string(rune('A'+i)),
			Status:    models.PinStatusQueued,
			Name:      "list-test",
			OwnerID:   ownerID,
			OwnerType: "user",
		}
		err := repo.Create(ctx, pin)
		if err != nil {
			t.Fatalf("Create() pin %d error = %v", i, err)
		}
	}

	// List all pins for owner
	pins, total, err := repo.ListByOwner(ctx, ownerID, "user", models.PinListOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListByOwner() error = %v", err)
	}

	if total < 3 {
		t.Errorf("expected at least 3 total, got %d", total)
	}
	if len(pins) < 3 {
		t.Errorf("expected at least 3 pins, got %d", len(pins))
	}
}

func TestPinRepository_ListByOwner_FilterByStatus(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	ownerID := "test-pin-listfilter-" + suffix

	// Create a queued pin
	queuedPin := &models.Pin{
		CID:       "QmQueued" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   ownerID,
		OwnerType: "user",
	}
	err := repo.Create(ctx, queuedPin)
	if err != nil {
		t.Fatalf("Create() queued pin error = %v", err)
	}

	// Create a pinned pin (create as queued, then update status)
	pinnedPin := &models.Pin{
		CID:       "QmPinned" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   ownerID,
		OwnerType: "user",
	}
	err = repo.Create(ctx, pinnedPin)
	if err != nil {
		t.Fatalf("Create() pinned pin error = %v", err)
	}
	err = repo.UpdateStatus(ctx, pinnedPin.ID, models.PinStatusPinned)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	// Filter by status=pinned
	pins, total, err := repo.ListByOwner(ctx, ownerID, "user", models.PinListOptions{
		Status: models.PinStatusPinned,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListByOwner() with status filter error = %v", err)
	}

	if total != 1 {
		t.Errorf("expected 1 pinned, got %d", total)
	}
	if len(pins) != 1 {
		t.Errorf("expected 1 pin in results, got %d", len(pins))
	}
	if len(pins) > 0 && pins[0].CID != "QmPinned"+suffix {
		t.Errorf("expected CID %q, got %q", "QmPinned"+suffix, pins[0].CID)
	}
}

func TestPinRepository_ListByOwner_Pagination(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	ownerID := "test-pin-paginate-" + suffix

	// Create 5 pins
	for i := 0; i < 5; i++ {
		pin := &models.Pin{
			CID:       "QmPage" + suffix + string(rune('A'+i)),
			Status:    models.PinStatusQueued,
			OwnerID:   ownerID,
			OwnerType: "user",
		}
		err := repo.Create(ctx, pin)
		if err != nil {
			t.Fatalf("Create() pin %d error = %v", i, err)
		}
	}

	// Get first page (limit 2)
	page1, total, err := repo.ListByOwner(ctx, ownerID, "user", models.PinListOptions{
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("ListByOwner() page 1 error = %v", err)
	}

	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(page1) != 2 {
		t.Errorf("expected 2 results on page 1, got %d", len(page1))
	}

	// Get second page
	page2, _, err := repo.ListByOwner(ctx, ownerID, "user", models.PinListOptions{
		Limit:  2,
		Offset: 2,
	})
	if err != nil {
		t.Fatalf("ListByOwner() page 2 error = %v", err)
	}

	if len(page2) != 2 {
		t.Errorf("expected 2 results on page 2, got %d", len(page2))
	}

	// Verify different results on each page
	if len(page1) > 0 && len(page2) > 0 && page1[0].ID == page2[0].ID {
		t.Error("expected different pins on different pages")
	}
}

func TestPinRepository_ListByOwner_FilterByCID(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	ownerID := "test-pin-filtercid-" + suffix

	targetCID := "QmFilterTarget" + suffix
	otherCID := "QmFilterOther" + suffix

	for _, cid := range []string{targetCID, otherCID} {
		pin := &models.Pin{
			CID:       cid,
			Status:    models.PinStatusQueued,
			OwnerID:   ownerID,
			OwnerType: "user",
		}
		err := repo.Create(ctx, pin)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// Filter by specific CID
	pins, total, err := repo.ListByOwner(ctx, ownerID, "user", models.PinListOptions{
		CID:   targetCID,
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListByOwner() with CID filter error = %v", err)
	}

	if total != 1 {
		t.Errorf("expected 1 result, got %d", total)
	}
	if len(pins) != 1 {
		t.Errorf("expected 1 pin, got %d", len(pins))
	}
	if len(pins) > 0 && pins[0].CID != targetCID {
		t.Errorf("expected CID %q, got %q", targetCID, pins[0].CID)
	}
}

func TestPinRepository_UpdateStatus(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	pin := &models.Pin{
		CID:       "QmUpdateStatus" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   "test-pin-update-" + suffix,
		OwnerType: "user",
	}
	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update from queued to pinning
	err = repo.UpdateStatus(ctx, pin.ID, models.PinStatusPinning)
	if err != nil {
		t.Fatalf("UpdateStatus() to pinning error = %v", err)
	}

	// Verify the update
	found, err := repo.GetByID(ctx, pin.ID)
	if err != nil {
		t.Fatalf("GetByID() after update error = %v", err)
	}
	if found.Status != models.PinStatusPinning {
		t.Errorf("expected status %q, got %q", models.PinStatusPinning, found.Status)
	}

	// Update from pinning to pinned
	err = repo.UpdateStatus(ctx, pin.ID, models.PinStatusPinned)
	if err != nil {
		t.Fatalf("UpdateStatus() to pinned error = %v", err)
	}

	found, err = repo.GetByID(ctx, pin.ID)
	if err != nil {
		t.Fatalf("GetByID() after pinned update error = %v", err)
	}
	if found.Status != models.PinStatusPinned {
		t.Errorf("expected status %q, got %q", models.PinStatusPinned, found.Status)
	}
	if found.PinnedAt == nil {
		t.Error("expected PinnedAt to be set when status is pinned")
	}
}

func TestPinRepository_UpdateStatus_NotFound(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()

	err := repo.UpdateStatus(ctx, "00000000-0000-0000-0000-000000000000", models.PinStatusPinned)
	if err != ErrPinNotFound {
		t.Errorf("expected ErrPinNotFound, got %v", err)
	}
}

func TestPinRepository_Delete(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	pin := &models.Pin{
		CID:       "QmDelete" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   "test-pin-delete-" + suffix,
		OwnerType: "user",
	}
	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete the pin
	err = repo.Delete(ctx, pin.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	_, err = repo.GetByID(ctx, pin.ID)
	if err != ErrPinNotFound {
		t.Errorf("expected ErrPinNotFound after delete, got %v", err)
	}
}

func TestPinRepository_Delete_NotFound(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()

	err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
	if err != ErrPinNotFound {
		t.Errorf("expected ErrPinNotFound, got %v", err)
	}
}

func TestPinRepository_ListByOwner_EmptyResult(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	// List for an owner with no pins
	pins, total, err := repo.ListByOwner(ctx, "test-pin-empty-"+suffix, "user", models.PinListOptions{
		Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListByOwner() error = %v", err)
	}

	if total != 0 {
		t.Errorf("expected 0 total, got %d", total)
	}
	if pins == nil {
		t.Error("expected non-nil empty slice, got nil")
	}
	if len(pins) != 0 {
		t.Errorf("expected 0 pins, got %d", len(pins))
	}
}

func TestPinRepository_ListByOwner_DefaultLimit(t *testing.T) {
	pool := setupPinTestDB(t)
	defer pool.Close()

	repo := NewPinRepository(pool)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")
	ownerID := "test-pin-deflimit-" + suffix

	// Create a pin
	pin := &models.Pin{
		CID:       "QmDefLimit" + suffix,
		Status:    models.PinStatusQueued,
		OwnerID:   ownerID,
		OwnerType: "user",
	}
	err := repo.Create(ctx, pin)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// List with 0 limit (should use default 10)
	pins, _, err := repo.ListByOwner(ctx, ownerID, "user", models.PinListOptions{
		Limit: 0,
	})
	if err != nil {
		t.Fatalf("ListByOwner() error = %v", err)
	}

	if len(pins) != 1 {
		t.Errorf("expected 1 pin with default limit, got %d", len(pins))
	}
}

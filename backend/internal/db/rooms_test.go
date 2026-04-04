package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// roomRepoTestCleanup deletes rooms with slugs matching a prefix for test isolation.
func roomRepoTestCleanup(ctx context.Context, pool *db.Pool, slugPrefix string) {
	pool.Exec(ctx, "DELETE FROM messages WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE $1)", slugPrefix+"%")       //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug LIKE $1)", slugPrefix+"%") //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM rooms WHERE slug LIKE $1", slugPrefix+"%")                                                  //nolint:errcheck
}

func TestRoomRepository_Create(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomcr"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	t.Run("creates room with auto-generated slug", func(t *testing.T) {
		params := models.CreateRoomParams{
			DisplayName: "Test Room Create",
			OwnerID:     uuid.Nil, // no owner
		}
		room, plainToken, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if room == nil {
			t.Fatal("Create() returned nil room")
		}
		if room.Slug == "" {
			t.Error("Create() returned empty slug")
		}
		if room.DisplayName != "Test Room Create" {
			t.Errorf("DisplayName = %q, want %q", room.DisplayName, "Test Room Create")
		}
		if room.MessageCount != 0 {
			t.Errorf("MessageCount = %d, want 0", room.MessageCount)
		}
		if plainToken == "" {
			t.Error("Create() returned empty token")
		}
		if room.TokenHash == "" {
			t.Error("Create() returned empty token_hash")
		}
		// Clean up
		pool.Exec(ctx, "DELETE FROM rooms WHERE id = $1", room.ID) //nolint:errcheck
	})
}

func TestRoomRepository_GetBySlug(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomgs"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	// Create a room to retrieve
	params := models.CreateRoomParams{
		DisplayName: "Get By Slug Room",
		OwnerID:     uuid.Nil,
	}
	created, _, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("returns room by slug", func(t *testing.T) {
		room, err := repo.GetBySlug(ctx, created.Slug)
		if err != nil {
			t.Fatalf("GetBySlug() error = %v", err)
		}
		if room.ID != created.ID {
			t.Errorf("ID = %v, want %v", room.ID, created.ID)
		}
	})

	t.Run("returns ErrRoomNotFound for non-existent slug", func(t *testing.T) {
		_, err := repo.GetBySlug(ctx, "non-existent-slug-xyz123")
		if err != db.ErrRoomNotFound {
			t.Errorf("GetBySlug() error = %v, want ErrRoomNotFound", err)
		}
	})
}

func TestRoomRepository_List(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomls"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	// Create 2 public rooms
	for i := 0; i < 2; i++ {
		params := models.CreateRoomParams{
			DisplayName: "List Room " + time.Now().Format("150405.000"),
			OwnerID:     uuid.Nil,
		}
		_, _, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		time.Sleep(10 * time.Millisecond) // ensure different timestamps
	}

	t.Run("returns public rooms with live_agent_count", func(t *testing.T) {
		rooms, err := repo.List(ctx, 10, 0)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(rooms) < 2 {
			t.Errorf("List() returned %d rooms, want >= 2", len(rooms))
		}
		// Each room should have LiveAgentCount field (even if 0)
		for _, r := range rooms {
			if r.LiveAgentCount < 0 {
				t.Errorf("LiveAgentCount = %d, want >= 0", r.LiveAgentCount)
			}
		}
	})

	t.Run("excludes soft-deleted rooms", func(t *testing.T) {
		// Create and then delete a room
		params := models.CreateRoomParams{
			DisplayName: "To Be Deleted",
			OwnerID:     uuid.Nil,
		}
		room, _, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		err = repo.SoftDelete(ctx, room.ID)
		if err != nil {
			t.Fatalf("SoftDelete() error = %v", err)
		}

		rooms, err := repo.List(ctx, 100, 0)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		for _, r := range rooms {
			if r.ID == room.ID {
				t.Error("List() returned soft-deleted room")
			}
		}
	})
}

func TestRoomRepository_Update(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomup"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	// Create room
	params := models.CreateRoomParams{
		DisplayName: "Update Test Room",
		OwnerID:     uuid.Nil,
	}
	room, _, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("updates display_name", func(t *testing.T) {
		newName := "Updated Name"
		updated, err := repo.Update(ctx, room.ID, models.UpdateRoomParams{
			DisplayName: &newName,
		})
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if updated.DisplayName != newName {
			t.Errorf("DisplayName = %q, want %q", updated.DisplayName, newName)
		}
		if !updated.UpdatedAt.After(room.UpdatedAt) {
			t.Error("UpdatedAt should be after original")
		}
	})
}

func TestRoomRepository_SoftDelete(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomsd"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	// Create room
	params := models.CreateRoomParams{
		DisplayName: "SoftDelete Room",
		OwnerID:     uuid.Nil,
	}
	room, _, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("sets deleted_at and GetBySlug returns not found", func(t *testing.T) {
		err := repo.SoftDelete(ctx, room.ID)
		if err != nil {
			t.Fatalf("SoftDelete() error = %v", err)
		}
		_, err = repo.GetBySlug(ctx, room.Slug)
		if err != db.ErrRoomNotFound {
			t.Errorf("GetBySlug() after delete: error = %v, want ErrRoomNotFound", err)
		}
	})
}

func TestRoomRepository_RotateToken(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomrt"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	// Create room
	params := models.CreateRoomParams{
		DisplayName: "Rotate Token Room",
		OwnerID:     uuid.Nil,
	}
	room, oldToken, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	t.Run("returns new token and old token no longer works", func(t *testing.T) {
		newToken, err := repo.RotateToken(ctx, room.ID)
		if err != nil {
			t.Fatalf("RotateToken() error = %v", err)
		}
		if newToken == "" {
			t.Error("RotateToken() returned empty token")
		}
		if newToken == oldToken {
			t.Error("RotateToken() returned same token")
		}
		// Verify old token doesn't match the new hash
		updated, err := repo.GetByID(ctx, room.ID)
		if err != nil {
			t.Fatalf("GetByID() error = %v", err)
		}
		if updated.TokenHash == room.TokenHash {
			t.Error("token_hash should have changed")
		}
	})
}

func TestRoomRepository_DeleteExpiredRooms(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomde"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	t.Run("deletes rooms past expires_at", func(t *testing.T) {
		// Create room with past expiry using direct SQL
		expiredSlug := "testroomde-exp-" + time.Now().Format("150405")
		_, err := pool.Exec(ctx, `
			INSERT INTO rooms (slug, display_name, token_hash, expires_at)
			VALUES ($1, 'Expired Room', 'hash_expired', NOW() - INTERVAL '1 hour')
		`, expiredSlug)
		if err != nil {
			t.Fatalf("insert expired room error = %v", err)
		}

		count, err := repo.DeleteExpiredRooms(ctx)
		if err != nil {
			t.Fatalf("DeleteExpiredRooms() error = %v", err)
		}
		if count < 1 {
			t.Errorf("DeleteExpiredRooms() deleted %d rooms, want >= 1", count)
		}
	})
}

func TestRoomRepository_ListByOwner(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	prefix := "testroomlo"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		roomRepoTestCleanup(cleanCtx, pool, prefix)
	})

	t.Run("returns only rooms owned by specified user", func(t *testing.T) {
		// Create room with a known owner using direct SQL
		ownerID := uuid.New()
		slug := "testroomlo-own-" + time.Now().Format("150405")

		// We need a real user for FK. Insert a test user first.
		refCode := time.Now().Format("15040500")[:8]
		_, err := pool.Exec(ctx, `
			INSERT INTO users (id, display_name, email, username, referral_code)
			VALUES ($1, 'Test Owner', $2, $3, $4)
			ON CONFLICT (id) DO NOTHING
		`, ownerID, "testowner-"+ownerID.String()[:8]+"@test.com", "to"+time.Now().Format("150405"), refCode)
		if err != nil {
			t.Fatalf("insert test user error = %v", err)
		}
		t.Cleanup(func() {
			cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cleanCancel()
			pool.Exec(cleanCtx, "DELETE FROM rooms WHERE owner_id = $1", ownerID)    //nolint:errcheck
			pool.Exec(cleanCtx, "DELETE FROM users WHERE id = $1", ownerID)           //nolint:errcheck
		})

		_, err = pool.Exec(ctx, `
			INSERT INTO rooms (slug, display_name, token_hash, owner_id)
			VALUES ($1, 'Owner Room', 'hash_owner', $2)
		`, slug, ownerID)
		if err != nil {
			t.Fatalf("insert owned room error = %v", err)
		}

		rooms, err := repo.ListByOwner(ctx, ownerID)
		if err != nil {
			t.Fatalf("ListByOwner() error = %v", err)
		}
		if len(rooms) < 1 {
			t.Errorf("ListByOwner() returned %d rooms, want >= 1", len(rooms))
		}
		for _, r := range rooms {
			if r.OwnerID == nil || *r.OwnerID != ownerID {
				t.Errorf("Room owner_id = %v, want %v", r.OwnerID, ownerID)
			}
		}
	})
}

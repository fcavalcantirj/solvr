package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

func TestRoomRepository_Archive_SetsArchivedState(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	prefix := "testroomarch1"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})

	repo := db.NewRoomRepository(pool)
	room, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-a", DisplayName: "Archive A"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// A room message captures the result.
	msgRepo := db.NewMessageRepository(pool)
	msg, err := msgRepo.Create(ctx, models.CreateMessageParams{
		RoomID:      room.ID,
		AuthorType:  "agent",
		AgentName:   "planner",
		Content:     "final result",
		ContentType: "text",
	})
	if err != nil {
		t.Fatalf("message Create() error = %v", err)
	}

	archived, err := repo.Archive(ctx, room.ID, &msg.ID)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("Archive() left archived_at nil")
	}
	if archived.ResultMessageID == nil || *archived.ResultMessageID != msg.ID {
		t.Errorf("ResultMessageID = %v, want %d", archived.ResultMessageID, msg.ID)
	}
	if !archived.IsArchived() {
		t.Error("IsArchived() = false after Archive()")
	}

	// Step 2: an archived room stays readable at its original slug.
	got, err := repo.GetBySlug(ctx, room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug() after archive error = %v", err)
	}
	if got.ArchivedAt == nil {
		t.Error("GetBySlug() did not surface archived_at")
	}
	if got.ResultMessageID == nil || *got.ResultMessageID != msg.ID {
		t.Errorf("GetBySlug ResultMessageID = %v, want %d", got.ResultMessageID, msg.ID)
	}
}

func TestRoomRepository_Archive_WithoutResultMessage(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	prefix := "testroomarch2"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})

	repo := db.NewRoomRepository(pool)
	room, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-a", DisplayName: "Archive NoResult"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	archived, err := repo.Archive(ctx, room.ID, nil)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if archived.ArchivedAt == nil {
		t.Fatal("Archive() left archived_at nil")
	}
	if archived.ResultMessageID != nil {
		t.Errorf("ResultMessageID = %v, want nil", *archived.ResultMessageID)
	}
}

func TestRoomRepository_Archive_PreservesTimestampOnReArchive(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	prefix := "testroomarch3"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})

	repo := db.NewRoomRepository(pool)
	room, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-a", DisplayName: "ReArchive"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	first, err := repo.Archive(ctx, room.ID, nil)
	if err != nil {
		t.Fatalf("Archive() first error = %v", err)
	}
	second, err := repo.Archive(ctx, room.ID, nil)
	if err != nil {
		t.Fatalf("Archive() second error = %v", err)
	}
	if !first.ArchivedAt.Equal(*second.ArchivedAt) {
		t.Errorf("re-archive changed archived_at: %v -> %v", first.ArchivedAt, second.ArchivedAt)
	}
}

func TestRoomRepository_Reopen_ClearsArchivedState(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	prefix := "testroomarch4"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})

	repo := db.NewRoomRepository(pool)
	room, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-a", DisplayName: "Reopen"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := repo.Archive(ctx, room.ID, nil); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	reopened, err := repo.Reopen(ctx, room.ID)
	if err != nil {
		t.Fatalf("Reopen() error = %v", err)
	}
	if reopened.ArchivedAt != nil {
		t.Errorf("Reopen() left archived_at = %v, want nil", reopened.ArchivedAt)
	}
	if reopened.IsArchived() {
		t.Error("IsArchived() = true after Reopen()")
	}

	got, err := repo.GetBySlug(ctx, room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug() after reopen error = %v", err)
	}
	if got.ArchivedAt != nil {
		t.Error("GetBySlug() still reports archived after reopen")
	}
}

func TestRoomRepository_Archive_NotFound(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	if _, err := repo.Archive(ctx, uuid.New(), nil); err != db.ErrRoomNotFound {
		t.Errorf("Archive(unknown) error = %v, want ErrRoomNotFound", err)
	}
	if _, err := repo.Reopen(ctx, uuid.New()); err != db.ErrRoomNotFound {
		t.Errorf("Reopen(unknown) error = %v, want ErrRoomNotFound", err)
	}
}

// Step 4: archiving is distinct from expiry and deletion. Archiving must not set
// deleted_at or expires_at, and a soft-deleted archived room disappears while an
// archived (not deleted) room stays readable.
func TestRoomRepository_Archive_DistinctFromDeleteAndExpiry(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	prefix := "testroomarch5"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})

	repo := db.NewRoomRepository(pool)
	room, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-a", DisplayName: "Distinct"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	archived, err := repo.Archive(ctx, room.ID, nil)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if archived.DeletedAt != nil {
		t.Error("Archive() set deleted_at (archiving must not delete)")
	}
	if archived.ExpiresAt != nil {
		t.Error("Archive() set expires_at (archiving must not expire)")
	}

	// Archived (not deleted) stays readable.
	if _, err := repo.GetBySlug(ctx, room.Slug); err != nil {
		t.Errorf("archived room not readable: %v", err)
	}

	// Soft delete removes it even though it is archived.
	if err := repo.SoftDelete(ctx, room.ID); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}
	if _, err := repo.GetBySlug(ctx, room.Slug); err != db.ErrRoomNotFound {
		t.Errorf("GetBySlug() after delete error = %v, want ErrRoomNotFound", err)
	}
}

// Step 5: starting a new collaboration from a finished room yields a wholly
// independent room — new identity, new token, no copied members or transcript.
func TestRoomRepository_StartNewFromFinished_Independent(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	prefix := "testroomarch6"
	roomRepoTestCleanup(ctx, pool, prefix)
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})

	repo := db.NewRoomRepository(pool)
	finished, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-old", DisplayName: "Finished"})
	if err != nil {
		t.Fatalf("Create(old) error = %v", err)
	}
	msgRepo := db.NewMessageRepository(pool)
	if _, err := msgRepo.Create(ctx, models.CreateMessageParams{
		RoomID: finished.ID, AuthorType: "agent", AgentName: "a", Content: "history", ContentType: "text",
	}); err != nil {
		t.Fatalf("seed message error = %v", err)
	}
	if _, err := repo.Archive(ctx, finished.ID, nil); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	fresh, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "-new", DisplayName: "Fresh"})
	if err != nil {
		t.Fatalf("Create(new) error = %v", err)
	}
	if fresh.ID == finished.ID || fresh.Slug == finished.Slug {
		t.Error("new room reused the finished room identity")
	}
	if fresh.MessageCount != 0 {
		t.Errorf("new room MessageCount = %d, want 0 (no copied transcript)", fresh.MessageCount)
	}
	if fresh.IsArchived() {
		t.Error("new room started archived")
	}
}

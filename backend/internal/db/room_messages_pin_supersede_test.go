package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestMessageRepository_PinUnpinList covers the review-loop pin primitive: an
// authorized participant pins a directive or result, participants list pinned
// entries without scanning history, and unpinning removes it.
func TestMessageRepository_PinUnpinList(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testpin-" + time.Now().Format("150405")
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewMessageRepository(pool)

	directive, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive: build the parser", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create directive: %v", err)
	}
	chatter, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "executor",
		Content: "on it", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create chatter: %v", err)
	}

	// Nothing pinned yet.
	pinned, err := repo.ListPinned(ctx, roomID)
	if err != nil {
		t.Fatalf("ListPinned (empty): %v", err)
	}
	if len(pinned) != 0 {
		t.Fatalf("expected 0 pinned, got %d", len(pinned))
	}

	// Pin the directive.
	got, err := repo.Pin(ctx, roomID, directive.ID)
	if err != nil {
		t.Fatalf("Pin: %v", err)
	}
	if got.PinnedAt == nil {
		t.Fatal("expected pinned_at set after Pin")
	}

	pinned, err = repo.ListPinned(ctx, roomID)
	if err != nil {
		t.Fatalf("ListPinned: %v", err)
	}
	if len(pinned) != 1 || pinned[0].ID != directive.ID {
		t.Fatalf("expected only the directive pinned, got %+v", pinned)
	}

	// The unpinned chatter must never surface via GetByID as pinned.
	fetched, err := repo.GetByID(ctx, roomID, chatter.ID)
	if err != nil {
		t.Fatalf("GetByID chatter: %v", err)
	}
	if fetched.PinnedAt != nil {
		t.Fatal("chatter must not be pinned")
	}

	// Unpin restores the empty pinned set.
	if _, err := repo.Unpin(ctx, roomID, directive.ID); err != nil {
		t.Fatalf("Unpin: %v", err)
	}
	pinned, err = repo.ListPinned(ctx, roomID)
	if err != nil {
		t.Fatalf("ListPinned after unpin: %v", err)
	}
	if len(pinned) != 0 {
		t.Fatalf("expected 0 pinned after unpin, got %d", len(pinned))
	}
}

// TestMessageRepository_Supersede covers posting a revised directive: the previous
// message stays in history and the new one carries an explicit superseding reference.
func TestMessageRepository_Supersede(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "testsuper-" + time.Now().Format("150405")
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})

	repo := db.NewMessageRepository(pool)

	original, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v1: use REST", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create original: %v", err)
	}

	revised, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v2: use SSE instead", ContentType: "text",
		SupersedesEntryID: &original.ID,
	})
	if err != nil {
		t.Fatalf("create revised: %v", err)
	}
	if revised.SupersedesEntryID == nil || *revised.SupersedesEntryID != original.ID {
		t.Fatalf("expected revised.supersedes_entry_id = %d, got %v", original.ID, revised.SupersedesEntryID)
	}

	// The original remains readable in history (not deleted, no supersede ref of its own).
	stillThere, err := repo.GetByID(ctx, roomID, original.ID)
	if err != nil {
		t.Fatalf("original must remain in history: %v", err)
	}
	if stillThere.SupersedesEntryID != nil {
		t.Fatal("original must not carry a superseding reference")
	}

	// Both appear in the ordered timeline.
	all, err := repo.ListRecent(ctx, roomID, 100)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected both messages retained, got %d", len(all))
	}
}

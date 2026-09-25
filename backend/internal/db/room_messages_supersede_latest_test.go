package db_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// A retried or stale revision must never fork or overwrite a newer directive: an
// entry that already has a live superseding entry can only be revised through its
// latest revision. The rule is enforced by the timeline trigger under the room lock,
// so it also holds for concurrent writers.

func supersedeLatestSetup(t *testing.T, prefix string) (context.Context, *db.Pool, uuid.UUID, *db.MessageRepository) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)

	slug := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		msgTestCleanup(cleanCtx, pool, slug)
	})
	return ctx, pool, roomID, db.NewMessageRepository(pool)
}

func revise(roomID uuid.UUID, target int64, content string) models.CreateMessageParams {
	return models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: content, ContentType: "text", SupersedesEntryID: &target,
	}
}

func TestMessageRepository_SupersedeOnlyTheLatestRevision(t *testing.T) {
	ctx, pool, roomID, repo := supersedeLatestSetup(t, "supl")

	v1, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v1", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	v2, err := repo.Create(ctx, revise(roomID, v1.ID, "Directive v2"))
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}

	// A stale revision of v1 (v2 already superseded it) is refused and stores nothing.
	if _, err := repo.Create(ctx, revise(roomID, v1.ID, "Directive v2 (stale fork)")); !errors.Is(err, db.ErrEntryAlreadySuperseded) {
		t.Fatalf("stale supersede of v1: err = %v, want ErrEntryAlreadySuperseded", err)
	}

	// Revising the latest revision is allowed.
	if _, err := repo.Create(ctx, revise(roomID, v2.ID, "Directive v3")); err != nil {
		t.Fatalf("create v3 superseding v2: %v", err)
	}

	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE room_id = $1`, roomID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("messages = %d, want 3 (the stale fork must not be stored)", n)
	}
}

func TestMessageRepository_DeletedRevisionFreesTheEntry(t *testing.T) {
	ctx, pool, roomID, repo := supersedeLatestSetup(t, "supd")

	v1, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v1", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	v2, err := repo.Create(ctx, revise(roomID, v1.ID, "Directive v2"))
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NOW() WHERE id = $1`, v2.ID); err != nil {
		t.Fatalf("delete v2: %v", err)
	}
	if _, err := repo.Create(ctx, revise(roomID, v1.ID, "Directive v2 again")); err != nil {
		t.Fatalf("supersede v1 after its revision was deleted: %v", err)
	}
}

func TestMessageRepository_SupersedeRetryReplays(t *testing.T) {
	ctx, _, roomID, repo := supersedeLatestSetup(t, "supr")

	v1, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v1", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	author, key := "planner-agent", "rev-2"
	params := revise(roomID, v1.ID, "Directive v2")
	params.AuthorID, params.ClientEntryID = &author, &key

	first, created, err := repo.CreateWithClientEntry(ctx, params)
	if err != nil || !created {
		t.Fatalf("first write: created=%v err=%v", created, err)
	}
	again, created, err := repo.CreateWithClientEntry(ctx, params)
	if err != nil {
		t.Fatalf("retry must replay, got err = %v", err)
	}
	if created || again.ID != first.ID {
		t.Fatalf("retry: created=%v id=%d, want replay of id %d", created, again.ID, first.ID)
	}
}

func TestMessageRepository_ConcurrentSupersedeHasOneWinner(t *testing.T) {
	ctx, _, roomID, repo := supersedeLatestSetup(t, "supc")

	v1, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v1", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}

	const writers = 8
	var wg sync.WaitGroup
	errs := make([]error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = repo.Create(ctx, revise(roomID, v1.ID, fmt.Sprintf("Directive v2 from writer %d", i)))
		}(i)
	}
	wg.Wait()

	wins, stale := 0, 0
	for _, e := range errs {
		switch {
		case e == nil:
			wins++
		case errors.Is(e, db.ErrEntryAlreadySuperseded):
			stale++
		default:
			t.Fatalf("unexpected error: %v", e)
		}
	}
	if wins != 1 || stale != writers-1 {
		t.Fatalf("wins=%d stale=%d, want exactly 1 winner and %d refused", wins, stale, writers-1)
	}
}

func TestMessageRepository_ConcurrentIdenticalSupersedeRetriesReplay(t *testing.T) {
	ctx, _, roomID, repo := supersedeLatestSetup(t, "supi")

	v1, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AgentName: "planner",
		Content: "Directive v1", ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	author, key := "planner-agent", "rev-2-concurrent"

	const retries = 8
	var wg sync.WaitGroup
	ids := make([]int64, retries)
	createdFlags := make([]bool, retries)
	errs := make([]error, retries)
	for i := 0; i < retries; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			params := revise(roomID, v1.ID, "Directive v2")
			params.AuthorID, params.ClientEntryID = &author, &key
			msg, created, err := repo.CreateWithClientEntry(ctx, params)
			errs[i], createdFlags[i] = err, created
			if msg != nil {
				ids[i] = msg.ID
			}
		}(i)
	}
	wg.Wait()

	createdCount := 0
	for i := 0; i < retries; i++ {
		if errs[i] != nil {
			t.Fatalf("retry %d: err = %v, want the stored revision replayed", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Fatalf("retry %d: id %d, want %d", i, ids[i], ids[0])
		}
		if createdFlags[i] {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created=%d, want exactly 1 new entry", createdCount)
	}
}

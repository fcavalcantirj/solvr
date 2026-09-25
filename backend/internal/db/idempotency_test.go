package db

import (
	"context"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func setupIdempotencyTest(t *testing.T) (*IdempotencyRepository, *Pool, string) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, url)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	actor := "idem-test-" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM idempotency_keys WHERE actor_id = $1", actor)
		pool.Close()
	})
	return NewIdempotencyRepository(pool), pool, actor
}

func TestIdempotencyRepository_ReserveCompleteReplay(t *testing.T) {
	repo, _, actor := setupIdempotencyTest(t)
	ctx := context.Background()
	scope := models.IdempotencyScope{ActorType: "agent", ActorID: actor, Operation: "post.create", Key: "k-1"}

	existing, reserved, err := repo.Reserve(ctx, scope, "hash-a")
	if err != nil || !reserved || existing != nil {
		t.Fatalf("first reserve: existing=%v reserved=%v err=%v, want nil/true/nil", existing, reserved, err)
	}
	existing, reserved, err = repo.Reserve(ctx, scope, "hash-a")
	if err != nil || reserved || existing == nil || existing.Status != models.IdempotencyPending {
		t.Fatalf("in-flight reserve: existing=%+v reserved=%v err=%v, want pending record", existing, reserved, err)
	}

	body := []byte(`{"data":{"id":"p1"}}`)
	if err := repo.Complete(ctx, scope, http.StatusCreated, "application/json", body); err != nil {
		t.Fatalf("complete: %v", err)
	}
	existing, reserved, err = repo.Reserve(ctx, scope, "hash-b")
	if err != nil || reserved {
		t.Fatalf("reserve after complete: reserved=%v err=%v", reserved, err)
	}
	if existing.Status != models.IdempotencyCompleted || existing.RequestHash != "hash-a" ||
		existing.ResponseStatus != http.StatusCreated || existing.ResponseContentType != "application/json" ||
		string(existing.ResponseBody) != string(body) {
		t.Fatalf("stored record = %+v", existing)
	}
}

func TestIdempotencyRepository_ReleaseFreesKey(t *testing.T) {
	repo, _, actor := setupIdempotencyTest(t)
	ctx := context.Background()
	scope := models.IdempotencyScope{ActorType: "human", ActorID: actor, Operation: "room.create", Key: "k-1"}

	if _, reserved, err := repo.Reserve(ctx, scope, "h"); err != nil || !reserved {
		t.Fatalf("reserve: %v %v", reserved, err)
	}
	if err := repo.Release(ctx, scope); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, reserved, err := repo.Reserve(ctx, scope, "h2"); err != nil || !reserved {
		t.Fatalf("reserve after release: reserved=%v err=%v, want true", reserved, err)
	}
}

func TestIdempotencyRepository_CompletedKeyRetainedWithinWindowTakenOverAfter(t *testing.T) {
	repo, pool, actor := setupIdempotencyTest(t)
	ctx := context.Background()
	scope := models.IdempotencyScope{ActorType: "agent", ActorID: actor, Operation: "reply.create", Key: "k-1"}
	if _, _, err := repo.Reserve(ctx, scope, "h"); err != nil {
		t.Fatal(err)
	}
	if err := repo.Complete(ctx, scope, 201, "application/json", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	// 23h old: still inside the 24h retention => replayed, not re-run.
	if _, err := pool.Exec(ctx, "UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '23 hours' WHERE actor_id = $1", actor); err != nil {
		t.Fatal(err)
	}
	if _, reserved, err := repo.Reserve(ctx, scope, "h"); err != nil || reserved {
		t.Fatalf("23h-old key: reserved=%v err=%v, want retained", reserved, err)
	}
	// 25h old: past retention => the key is free again.
	if _, err := pool.Exec(ctx, "UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '25 hours' WHERE actor_id = $1", actor); err != nil {
		t.Fatal(err)
	}
	if _, reserved, err := repo.Reserve(ctx, scope, "h-new"); err != nil || !reserved {
		t.Fatalf("25h-old key: reserved=%v err=%v, want taken over", reserved, err)
	}
}

func TestIdempotencyRepository_StalePendingTakenOver(t *testing.T) {
	repo, pool, actor := setupIdempotencyTest(t)
	ctx := context.Background()
	scope := models.IdempotencyScope{ActorType: "agent", ActorID: actor, Operation: "post.create", Key: "k-1"}
	if _, _, err := repo.Reserve(ctx, scope, "h"); err != nil {
		t.Fatal(err)
	}
	// A pending row whose server died is not locked for the whole retention window.
	if _, err := pool.Exec(ctx, "UPDATE idempotency_keys SET created_at = NOW() - INTERVAL '10 minutes' WHERE actor_id = $1", actor); err != nil {
		t.Fatal(err)
	}
	if _, reserved, err := repo.Reserve(ctx, scope, "h"); err != nil || !reserved {
		t.Fatalf("stale pending: reserved=%v err=%v, want taken over", reserved, err)
	}
}

func TestIdempotencyRepository_ConcurrentReserveHasOneWinner(t *testing.T) {
	repo, _, actor := setupIdempotencyTest(t)
	ctx := context.Background()
	scope := models.IdempotencyScope{ActorType: "agent", ActorID: actor, Operation: "post.create", Key: "race"}

	const n = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, reserved, err := repo.Reserve(ctx, scope, "h")
			if err != nil {
				t.Errorf("reserve: %v", err)
				return
			}
			if reserved {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if winners != 1 {
		t.Fatalf("%d concurrent reservations won, want exactly 1", winners)
	}
}

func TestIdempotencyRepository_DeleteExpiredPrunesOnlyPastRetention(t *testing.T) {
	repo, pool, actor := setupIdempotencyTest(t)
	ctx := context.Background()
	scope := func(key string) models.IdempotencyScope {
		return models.IdempotencyScope{ActorType: "agent", ActorID: actor, Operation: "post.create", Key: key}
	}
	for _, key := range []string{"old-completed", "old-pending", "recent-completed", "fresh-pending"} {
		if _, reserved, err := repo.Reserve(ctx, scope(key), "h"); err != nil || !reserved {
			t.Fatalf("reserve %s: reserved=%v err=%v", key, reserved, err)
		}
	}
	for _, key := range []string{"old-completed", "recent-completed"} {
		if err := repo.Complete(ctx, scope(key), http.StatusCreated, "application/json", []byte(`{}`)); err != nil {
			t.Fatalf("complete %s: %v", key, err)
		}
	}
	age := map[string]string{"old-completed": "25 hours", "old-pending": "25 hours", "recent-completed": "23 hours"}
	for key, ago := range age {
		if _, err := pool.Exec(ctx, "UPDATE idempotency_keys SET created_at = NOW() - $3::interval WHERE actor_id = $1 AND idempotency_key = $2", actor, key, ago); err != nil {
			t.Fatalf("age %s: %v", key, err)
		}
	}

	deleted, err := repo.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}
	if deleted < 2 {
		t.Fatalf("deleted = %d, want at least this test's 2 expired rows", deleted)
	}

	rows, err := pool.Query(ctx, "SELECT idempotency_key FROM idempotency_keys WHERE actor_id = $1 ORDER BY idempotency_key", actor)
	if err != nil {
		t.Fatalf("query remaining: %v", err)
	}
	defer rows.Close()
	var remaining []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatalf("scan: %v", err)
		}
		remaining = append(remaining, k)
	}
	if len(remaining) != 2 || remaining[0] != "fresh-pending" || remaining[1] != "recent-completed" {
		t.Fatalf("remaining keys = %v, want [fresh-pending recent-completed]", remaining)
	}

	// The pruned key is immediately reusable as a brand-new request.
	if _, reserved, err := repo.Reserve(ctx, scope("old-completed"), "h2"); err != nil || !reserved {
		t.Fatalf("reserve pruned key: reserved=%v err=%v, want reserved", reserved, err)
	}
}

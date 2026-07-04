package db_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestRoomClaimRepository_ConcurrentAcquire_ExactlyOneWins is the core atomicity AC:
// many callers racing for the same key -> exactly one wins.
func TestRoomClaimRepository_ConcurrentAcquire_ExactlyOneWins(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-claim-race", false)
	repo := db.NewRoomClaimRepository(pool)

	const n = 24
	var wins int32
	var mu sync.Mutex
	var winners []string
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		holder := "worker-" + string(rune('A'+i))
		go func(h string) {
			defer wg.Done()
			<-start // release all goroutines at once for maximum contention
			claim, won, err := repo.Acquire(ctx, models.AcquireClaimParams{
				RoomID: room.ID, Key: "APP-185", Holder: h, TTLSeconds: 300,
			})
			if err != nil {
				t.Errorf("Acquire error: %v", err)
				return
			}
			if won {
				mu.Lock()
				wins++
				winners = append(winners, h)
				mu.Unlock()
			} else if claim == nil {
				t.Errorf("held outcome returned nil claim")
			}
		}(holder)
	}
	close(start)
	wg.Wait()

	if wins != 1 {
		t.Fatalf("expected exactly 1 winner, got %d (%v)", wins, winners)
	}

	// The single live claim reflects the winner.
	live, err := repo.ListLive(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListLive: %v", err)
	}
	if len(live) != 1 || live[0].Key != "APP-185" || live[0].Holder != winners[0] {
		t.Fatalf("live claims = %+v; want single claim held by %s", live, winners[0])
	}
}

func TestRoomClaimRepository_LiveClaim_SecondCallerHeld(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-claim-held", false)
	repo := db.NewRoomClaimRepository(pool)

	if _, won, err := repo.Acquire(ctx, models.AcquireClaimParams{RoomID: room.ID, Key: "k1", Holder: "alice", TTLSeconds: 300}); err != nil || !won {
		t.Fatalf("first Acquire won=%v err=%v; want won", won, err)
	}
	claim, won, err := repo.Acquire(ctx, models.AcquireClaimParams{RoomID: room.ID, Key: "k1", Holder: "bob", TTLSeconds: 300})
	if err != nil {
		t.Fatalf("second Acquire err=%v", err)
	}
	if won {
		t.Fatalf("second caller unexpectedly won")
	}
	if claim.Holder != "alice" {
		t.Fatalf("held claim holder = %q; want alice", claim.Holder)
	}
}

func TestRoomClaimRepository_ExpiredClaim_CanBeStolen(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-claim-expire", false)
	repo := db.NewRoomClaimRepository(pool)

	if _, won, err := repo.Acquire(ctx, models.AcquireClaimParams{RoomID: room.ID, Key: "k1", Holder: "alice", TTLSeconds: 300}); err != nil || !won {
		t.Fatalf("first Acquire won=%v err=%v", won, err)
	}
	// Force-expire alice's claim deterministically (no sleeping).
	if _, err := pool.Exec(ctx, `UPDATE room_claims SET expires_at = NOW() - interval '1 second' WHERE room_id = $1 AND claim_key = 'k1'`, room.ID); err != nil {
		t.Fatalf("force expire: %v", err)
	}
	// It no longer counts as live.
	if live, _ := repo.ListLive(ctx, room.ID); len(live) != 0 {
		t.Fatalf("expected 0 live claims after expiry, got %d", len(live))
	}
	// Bob steals the expired lock.
	claim, won, err := repo.Acquire(ctx, models.AcquireClaimParams{RoomID: room.ID, Key: "k1", Holder: "bob", TTLSeconds: 300})
	if err != nil || !won {
		t.Fatalf("steal Acquire won=%v err=%v; want won", won, err)
	}
	if claim.Holder != "bob" {
		t.Fatalf("stolen claim holder = %q; want bob", claim.Holder)
	}
}

func TestRoomClaimRepository_RenewAndRelease(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-claim-renew", false)
	repo := db.NewRoomClaimRepository(pool)

	first, won, err := repo.Acquire(ctx, models.AcquireClaimParams{RoomID: room.ID, Key: "k1", Holder: "alice", TTLSeconds: 60})
	if err != nil || !won {
		t.Fatalf("Acquire won=%v err=%v", won, err)
	}

	// Renew by the holder extends the expiry.
	renewed, err := repo.Renew(ctx, room.ID, "k1", "alice", 600)
	if err != nil {
		t.Fatalf("Renew: %v", err)
	}
	if !renewed.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("renew did not extend expiry: %v <= %v", renewed.ExpiresAt, first.ExpiresAt)
	}

	// A non-holder cannot renew.
	if _, err := repo.Renew(ctx, room.ID, "k1", "bob", 600); err != db.ErrClaimNotHeld {
		t.Fatalf("Renew(non-holder) err = %v; want ErrClaimNotHeld", err)
	}
	// A non-holder cannot release.
	if err := repo.Release(ctx, room.ID, "k1", "bob"); err != db.ErrClaimNotHeld {
		t.Fatalf("Release(non-holder) err = %v; want ErrClaimNotHeld", err)
	}
	// The holder releases -> key is free and can be re-acquired by anyone.
	if err := repo.Release(ctx, room.ID, "k1", "alice"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if _, won, err := repo.Acquire(ctx, models.AcquireClaimParams{RoomID: room.ID, Key: "k1", Holder: "bob", TTLSeconds: 60}); err != nil || !won {
		t.Fatalf("re-Acquire after release won=%v err=%v; want won", won, err)
	}
}

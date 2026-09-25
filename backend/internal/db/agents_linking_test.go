package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// LinkHuman is the single gate of a claim: it links an unclaimed agent once. Linking again,
// by the same human or in parallel, reports ErrAgentAlreadyClaimed and leaves the first
// link (and its human_claimed_at) untouched.
func TestAgentRepository_LinkHuman_LinksOnce(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()
	newAgent := func(prefix string) string {
		now := time.Now()
		agent := &models.Agent{
			ID:          prefix + now.Format("150405") + fmt.Sprintf("%06d", now.Nanosecond()/1000),
			DisplayName: "Link Once " + prefix + now.Format("150405.000000"),
		}
		if err := repo.Create(ctx, agent); err != nil {
			t.Fatalf("create agent: %v", err)
		}
		t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM agents WHERE id = $1", agent.ID) }) //nolint:errcheck
		return agent.ID
	}

	t.Run("same human again", func(t *testing.T) {
		agentID := newAgent("lnkonce_")
		humanID := createAgentTestUser(t, pool)
		if err := repo.LinkHuman(ctx, agentID, humanID); err != nil {
			t.Fatalf("first link: %v", err)
		}
		first, err := repo.FindByID(ctx, agentID)
		if err != nil {
			t.Fatalf("find: %v", err)
		}
		if err := repo.LinkHuman(ctx, agentID, humanID); err != ErrAgentAlreadyClaimed {
			t.Fatalf("second link by the same human: expected ErrAgentAlreadyClaimed, got %v", err)
		}
		again, err := repo.FindByID(ctx, agentID)
		if err != nil {
			t.Fatalf("find: %v", err)
		}
		if again.HumanID == nil || *again.HumanID != humanID {
			t.Fatalf("expected human_id %s, got %v", humanID, again.HumanID)
		}
		if first.HumanClaimedAt == nil || again.HumanClaimedAt == nil || !again.HumanClaimedAt.Equal(*first.HumanClaimedAt) {
			t.Fatalf("human_claimed_at must not move: %v -> %v", first.HumanClaimedAt, again.HumanClaimedAt)
		}
	})

	t.Run("parallel links", func(t *testing.T) {
		agentID := newAgent("lnkpar_")
		humans := []string{createAgentTestUser(t, pool), createAgentTestUser(t, pool)}
		const n = 12
		errs := make([]error, n)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range n {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				errs[i] = repo.LinkHuman(ctx, agentID, humans[i%len(humans)])
			}()
		}
		close(start)
		wg.Wait()

		winners := 0
		for i, err := range errs {
			switch err {
			case nil:
				winners++
			case ErrAgentAlreadyClaimed:
			default:
				t.Fatalf("link %d: expected nil or ErrAgentAlreadyClaimed, got %v", i, err)
			}
		}
		if winners != 1 {
			t.Fatalf("expected exactly one link to succeed, got %d (%v)", winners, errs)
		}
	})
}

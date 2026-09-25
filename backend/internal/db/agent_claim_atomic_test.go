package db

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

func TestClaimTokenRepository_ClaimAgentAtomically(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	ctx := context.Background()
	agents := NewAgentRepository(pool)
	claims := NewClaimTokenRepository(pool)
	newFixture := func(t *testing.T, prefix string) (*models.Agent, *models.ClaimToken, string) {
		t.Helper()
		suffix := fmt.Sprintf("%d", time.Now().UnixNano())
		agent := &models.Agent{
			ID:          prefix + suffix,
			DisplayName: "Atomic Claim " + suffix,
		}
		if err := agents.Create(ctx, agent); err != nil {
			t.Fatalf("create agent: %v", err)
		}
		humanID := createAgentTestUser(t, pool)
		token := &models.ClaimToken{
			Token:     "atomic_claim_" + suffix,
			AgentID:   agent.ID,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := claims.Create(ctx, token); err != nil {
			t.Fatalf("create token: %v", err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), "DELETE FROM claim_tokens WHERE agent_id = $1", agent.ID) //nolint:errcheck
			pool.Exec(context.Background(), "DELETE FROM agents WHERE id = $1", agent.ID)             //nolint:errcheck
		})
		return agent, token, humanID
	}

	t.Run("commits link bonus badge and token together", func(t *testing.T) {
		agent, token, humanID := newFixture(t, "claim_commit_")
		if err := claims.ClaimAgent(ctx, token.ID, agent.ID, humanID, 50); err != nil {
			t.Fatalf("ClaimAgent: %v", err)
		}

		got, err := agents.FindByID(ctx, agent.ID)
		if err != nil {
			t.Fatalf("find claimed agent: %v", err)
		}
		if got.HumanID == nil || *got.HumanID != humanID {
			t.Fatalf("human_id = %v, want %s", got.HumanID, humanID)
		}
		if got.Reputation != 50 || !got.HasHumanBackedBadge {
			t.Fatalf("reputation/badge = %d/%v, want 50/true", got.Reputation, got.HasHumanBackedBadge)
		}
		used, err := claims.FindByToken(ctx, token.Token)
		if err != nil {
			t.Fatalf("find used token: %v", err)
		}
		if used.UsedAt == nil || used.UsedByHumanID == nil || *used.UsedByHumanID != humanID {
			t.Fatalf("token use = at:%v by:%v, want used by %s", used.UsedAt, used.UsedByHumanID, humanID)
		}
	})

	t.Run("rolls back agent changes when token cannot be consumed", func(t *testing.T) {
		agent, token, humanID := newFixture(t, "claim_rollback_")
		_, err := pool.Exec(ctx, "DELETE FROM claim_tokens WHERE id = $1", token.ID)
		if err != nil {
			t.Fatalf("remove token: %v", err)
		}

		err = claims.ClaimAgent(ctx, token.ID, agent.ID, humanID, 50)
		if !errors.Is(err, ErrClaimTokenNotFound) {
			t.Fatalf("ClaimAgent error = %v, want ErrClaimTokenNotFound", err)
		}
		got, err := agents.FindByID(ctx, agent.ID)
		if err != nil {
			t.Fatalf("find agent after rollback: %v", err)
		}
		if got.HumanID != nil || got.Reputation != 0 || got.HasHumanBackedBadge {
			t.Fatalf("claim partially committed: human=%v reputation=%d badge=%v", got.HumanID, got.Reputation, got.HasHumanBackedBadge)
		}
	})
}

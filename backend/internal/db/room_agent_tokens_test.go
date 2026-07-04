package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/token"
)

func TestRoomAgentTokenRepository_IssueResolveExpireRevoke(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-agent-tok", true)
	insertTestAgentForMembers(ctx, t, pool, "agent_tok_x")
	repo := db.NewRoomAgentTokenRepository(pool)

	// Non-expiring token resolves to (room, agent).
	tok, err := repo.Issue(ctx, room.ID, "agent_tok_x", 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	id, err := repo.ResolveByHash(ctx, token.HashToken(tok))
	if err != nil {
		t.Fatalf("ResolveByHash: %v", err)
	}
	if id.RoomID != room.ID || id.AgentID != "agent_tok_x" {
		t.Fatalf("resolved identity = %+v; want room=%s agent=agent_tok_x", id, room.ID)
	}

	// Re-issuing replaces the previous token (old hash no longer resolves).
	tok2, err := repo.Issue(ctx, room.ID, "agent_tok_x", 3600)
	if err != nil {
		t.Fatalf("Issue #2: %v", err)
	}
	if _, err := repo.ResolveByHash(ctx, token.HashToken(tok)); err != db.ErrAgentRoomTokenNotFound {
		t.Fatalf("old token still resolves; err=%v want ErrAgentRoomTokenNotFound", err)
	}
	if _, err := repo.ResolveByHash(ctx, token.HashToken(tok2)); err != nil {
		t.Fatalf("new token should resolve: %v", err)
	}

	// Force-expire the token -> resolve fails.
	if _, err := pool.Exec(ctx, `UPDATE room_agent_tokens SET expires_at = NOW() - interval '1 minute' WHERE room_id = $1`, room.ID); err != nil {
		t.Fatalf("force expire: %v", err)
	}
	if _, err := repo.ResolveByHash(ctx, token.HashToken(tok2)); err != db.ErrAgentRoomTokenNotFound {
		t.Fatalf("expired token resolves; err=%v want ErrAgentRoomTokenNotFound", err)
	}

	// Revoke removes the row entirely (idempotent).
	if err := repo.Revoke(ctx, room.ID, "agent_tok_x"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := repo.Revoke(ctx, room.ID, "agent_tok_x"); err != nil {
		t.Fatalf("Revoke (idempotent): %v", err)
	}
}

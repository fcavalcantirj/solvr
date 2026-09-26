package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/jackc/pgx/v5"
)

// idx 75 step 3: revocation must reach credentials that were valid when they were issued.
// A per-agent room token proves admission to a room, not that its agent still exists, so a
// token of a soft-deleted agent must stop resolving (REST, /r adapter, tickets, open streams
// all go through ResolveByHash / IsLive), and the deletion must be announced to every open
// stream of the rooms the account was in (000106).

func softDeleteAgent(ctx context.Context, t *testing.T, pool *db.Pool, agentID string) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE agents SET deleted_at = NOW() WHERE id = $1`, agentID); err != nil {
		t.Fatalf("soft delete agent: %v", err)
	}
}

func TestRoomAgentTokenRepository_ADeletedAgentsTokenStopsResolving(t *testing.T) {
	ctx, pool, repo, room := sessionTokenFixture(t, "rm-tok-deleted-agent", "agent_tok_deleted")
	roomID := room()
	plaintext, err := repo.Issue(ctx, roomID, "agent_tok_deleted", 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	hash := token.HashToken(plaintext)
	if id, err := repo.ResolveByHash(ctx, hash); err != nil || id.AgentID != "agent_tok_deleted" {
		t.Fatalf("a live agent's token resolves: %v %+v", err, id)
	}

	softDeleteAgent(ctx, t, pool, "agent_tok_deleted")

	if _, err := repo.ResolveByHash(ctx, hash); !errors.Is(err, db.ErrAgentRoomTokenNotFound) {
		t.Fatalf("a deleted agent's token must not resolve: err = %v", err)
	}
	if live, err := repo.IsLive(ctx, hash, roomID); err != nil || live {
		t.Fatalf("a deleted agent's token must not be live for an open stream: live=%v err=%v", live, err)
	}
	if rotated, err := repo.WasRotated(ctx, hash); err != nil || rotated {
		t.Fatalf("a deletion is not a rotation, the holder must not be told to handshake again: %v %v", rotated, err)
	}

	// An admin recovering the account restores what the account held.
	if _, err := pool.Exec(ctx, `UPDATE agents SET deleted_at = NULL WHERE id = $1`, "agent_tok_deleted"); err != nil {
		t.Fatalf("restore agent: %v", err)
	}
	if id, err := repo.ResolveByHash(ctx, hash); err != nil || id.AgentID != "agent_tok_deleted" {
		t.Fatalf("a recovered agent's token resolves again: %v %+v", err, id)
	}
}

// roomAccessNotices LISTENs on the channel the API instances hear room access changes on,
// and returns a function that collects the room ids announced within the window.
func roomAccessNotices(ctx context.Context, t *testing.T) func(window time.Duration) map[string]bool {
	t.Helper()
	conn, err := pgx.Connect(ctx, getTestDatabaseURL(t))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	if _, err := conn.Exec(ctx, "LISTEN "+db.RoomAccessChannel); err != nil {
		t.Fatalf("LISTEN: %v", err)
	}
	return func(window time.Duration) map[string]bool {
		got := map[string]bool{}
		deadline := time.Now().Add(window)
		for {
			wctx, cancel := context.WithDeadline(ctx, deadline)
			n, err := conn.WaitForNotification(wctx)
			cancel()
			if err != nil {
				return got
			}
			got[n.Payload] = true
		}
	}
}

func TestAccountDeletion_AnnouncesRoomAccessToTheRoomsTheAccountWasIn(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	notices := roomAccessNotices(ctx, t)

	// An agent that is a member of two rooms (holding a token in one: a token row implies an
	// active membership), a room it is unrelated to, and a human who is the sole owner of a
	// private room.
	withToken := createMemberTestRoom(ctx, t, pool, "rm-notify-token", true)
	memberOnly := createMemberTestRoom(ctx, t, pool, "rm-notify-member", true)
	unrelated := createMemberTestRoom(ctx, t, pool, "rm-notify-unrelated", true)
	admitPresenceMember(ctx, t, pool, withToken.ID, "agent_notify_gone")
	admitPresenceMember(ctx, t, pool, memberOnly.ID, "agent_notify_gone")
	if _, err := db.NewRoomAgentTokenRepository(pool).Issue(ctx, withToken.ID, "agent_notify_gone", 0); err != nil {
		t.Fatalf("issue token: %v", err)
	}
	user := insertAuthorityUser(ctx, t, pool, "notifyowner")
	owned := createAuthorityRoom(ctx, t, pool, "rm-notify-owned", models.CreateRoomParams{IsPrivate: true, OwnerID: user})
	notices(300 * time.Millisecond) // drain what the setup announced

	softDeleteAgent(ctx, t, pool, "agent_notify_gone")
	got := notices(1500 * time.Millisecond)
	if !got[withToken.ID.String()] || !got[memberOnly.ID.String()] {
		t.Fatalf("deleting an agent must announce every room it is a member of: %v", got)
	}
	if got[unrelated.ID.String()] || got[owned.ID.String()] {
		t.Fatalf("a room the agent was never in must not be announced: %v", got)
	}

	softDeleteUser(ctx, t, pool, user)
	got = notices(1500 * time.Millisecond)
	if !got[owned.ID.String()] {
		t.Fatalf("deleting a human must announce the room it solely owned (its owner row stays active for recovery, so nothing else announces it): %v", got)
	}
	if got[unrelated.ID.String()] {
		t.Fatalf("a room the human was never in must not be announced: %v", got)
	}
}

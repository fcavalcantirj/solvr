package db_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/google/uuid"
)

// idx 75 step 1: one agent may run several sessions in a room. A handshake ADDS a token;
// only an explicit rotation replaces the others, and a token replaced that way is
// remembered so its holder gets a recoverable answer instead of a bare "invalid token".

func sessionTokenFixture(t *testing.T, slug, agentID string) (context.Context, *db.Pool, *db.RoomAgentTokenRepository, func() uuid.UUID) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	room := createMemberTestRoom(ctx, t, pool, slug, true)
	admitPresenceMember(ctx, t, pool, room.ID, agentID)
	repo := db.NewRoomAgentTokenRepository(pool)
	return ctx, pool, repo, func() uuid.UUID { return room.ID }
}

func liveTokens(ctx context.Context, t *testing.T, pool *db.Pool, roomID uuid.UUID, agentID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_agent_tokens
		WHERE room_id = $1 AND agent_id = $2 AND rotated_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`,
		roomID, agentID).Scan(&n); err != nil {
		t.Fatalf("count live tokens: %v", err)
	}
	return n
}

func TestRoomAgentTokenRepository_IssueAddsASessionAndKeepsTheOthers(t *testing.T) {
	ctx, _, repo, room := sessionTokenFixture(t, "rm-tok-sessions-add", "agent_tok_sess_add")
	roomID := room()

	first, err := repo.Issue(ctx, roomID, "agent_tok_sess_add", 0)
	if err != nil {
		t.Fatalf("Issue #1: %v", err)
	}
	second, err := repo.Issue(ctx, roomID, "agent_tok_sess_add", 3600)
	if err != nil {
		t.Fatalf("Issue #2: %v", err)
	}
	if first == second {
		t.Fatal("two handshakes issued the same token")
	}
	for name, plaintext := range map[string]string{"first": first, "second": second} {
		id, err := repo.ResolveByHash(ctx, token.HashToken(plaintext))
		if err != nil || id.AgentID != "agent_tok_sess_add" {
			t.Fatalf("%s session's token must keep resolving after the other handshake: %v %+v", name, err, id)
		}
		if live, err := repo.IsLive(ctx, token.HashToken(plaintext), roomID); err != nil || !live {
			t.Fatalf("%s session's token must stay live: %v %v", name, live, err)
		}
		if rotated, err := repo.WasRotated(ctx, token.HashToken(plaintext)); err != nil || rotated {
			t.Fatalf("%s session's token was never rotated: %v %v", name, rotated, err)
		}
	}
}

func TestRoomAgentTokenRepository_RotateReplacesEverySessionAndRemembersThem(t *testing.T) {
	ctx, _, repo, room := sessionTokenFixture(t, "rm-tok-sessions-rot", "agent_tok_sess_rot")
	roomID := room()

	a, _ := repo.Issue(ctx, roomID, "agent_tok_sess_rot", 0)
	b, _ := repo.Issue(ctx, roomID, "agent_tok_sess_rot", 0)
	c, replaced, err := repo.Rotate(ctx, roomID, "agent_tok_sess_rot", 0)
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if replaced != 2 {
		t.Fatalf("Rotate replaced %d live tokens; want 2", replaced)
	}

	for name, plaintext := range map[string]string{"a": a, "b": b} {
		h := token.HashToken(plaintext)
		if _, err := repo.ResolveByHash(ctx, h); !errors.Is(err, db.ErrAgentRoomTokenNotFound) {
			t.Fatalf("session %s must not resolve after the rotation, err=%v", name, err)
		}
		if live, _ := repo.IsLive(ctx, h, roomID); live {
			t.Fatalf("session %s must not be live after the rotation", name)
		}
		if rotated, err := repo.WasRotated(ctx, h); err != nil || !rotated {
			t.Fatalf("session %s was replaced by a rotation and must say so: %v %v", name, rotated, err)
		}
	}
	if _, err := repo.ResolveByHash(ctx, token.HashToken(c)); err != nil {
		t.Fatalf("the rotated-in token must resolve: %v", err)
	}
	if rotated, _ := repo.WasRotated(ctx, token.HashToken(c)); rotated {
		t.Fatal("the new token is not a rotated one")
	}
	if rotated, _ := repo.WasRotated(ctx, token.HashToken("solvr_rt_never_issued")); rotated {
		t.Fatal("a token that never existed is not a rotated one")
	}
}

func TestRoomAgentTokenRepository_RotateOnAnAgentWithNoTokenJustIssues(t *testing.T) {
	ctx, _, repo, room := sessionTokenFixture(t, "rm-tok-sessions-first", "agent_tok_sess_first")
	tok, replaced, err := repo.Rotate(ctx, room(), "agent_tok_sess_first", 0)
	if err != nil {
		t.Fatalf("Rotate with nothing to replace: %v", err)
	}
	if replaced != 0 {
		t.Fatalf("Rotate replaced %d tokens of an agent that held none", replaced)
	}
	if _, err := repo.ResolveByHash(ctx, token.HashToken(tok)); err != nil {
		t.Fatalf("token must resolve: %v", err)
	}
}

func TestRoomAgentTokenRepository_LiveTokensAreCappedAndRotationClearsTheCap(t *testing.T) {
	ctx, pool, repo, room := sessionTokenFixture(t, "rm-tok-sessions-cap", "agent_tok_sess_cap")
	roomID := room()

	for i := 0; i < db.MaxLiveRoomAgentTokens; i++ {
		if _, err := repo.Issue(ctx, roomID, "agent_tok_sess_cap", 0); err != nil {
			t.Fatalf("Issue %d of %d: %v", i+1, db.MaxLiveRoomAgentTokens, err)
		}
	}
	if _, err := repo.Issue(ctx, roomID, "agent_tok_sess_cap", 0); !errors.Is(err, db.ErrAgentRoomTokenLimit) {
		t.Fatalf("the token past the cap must be ErrAgentRoomTokenLimit, got %v", err)
	}

	// An expired token is not a live session and frees its slot.
	if _, err := pool.Exec(ctx, `UPDATE room_agent_tokens SET expires_at = NOW() - interval '1 minute'
		WHERE token_hash = (SELECT token_hash FROM room_agent_tokens WHERE room_id = $1 AND agent_id = $2 LIMIT 1)`,
		roomID, "agent_tok_sess_cap"); err != nil {
		t.Fatalf("expire one: %v", err)
	}
	if _, err := repo.Issue(ctx, roomID, "agent_tok_sess_cap", 0); err != nil {
		t.Fatalf("an expired session frees a slot: %v", err)
	}
	if _, err := repo.Issue(ctx, roomID, "agent_tok_sess_cap", 0); !errors.Is(err, db.ErrAgentRoomTokenLimit) {
		t.Fatalf("full again: want ErrAgentRoomTokenLimit, got %v", err)
	}

	// Rotation is the explicit way out and is never refused for being at the cap.
	if _, _, err := repo.Rotate(ctx, roomID, "agent_tok_sess_cap", 0); err != nil {
		t.Fatalf("Rotate at the cap: %v", err)
	}
	if n := liveTokens(ctx, t, pool, roomID, "agent_tok_sess_cap"); n != 1 {
		t.Fatalf("live tokens after rotation = %d; want exactly 1", n)
	}
}

func TestRoomAgentTokenRepository_ConcurrentHandshakesNeverExceedTheCap(t *testing.T) {
	ctx, pool, repo, room := sessionTokenFixture(t, "rm-tok-sessions-race", "agent_tok_sess_race")
	roomID := room()

	const attempts = 3 * db.MaxLiveRoomAgentTokens
	var wg sync.WaitGroup
	var mu sync.Mutex
	issued, refused := 0, 0
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Issue(ctx, roomID, "agent_tok_sess_race", 0)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				issued++
			case errors.Is(err, db.ErrAgentRoomTokenLimit):
				refused++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if issued != db.MaxLiveRoomAgentTokens || refused != attempts-db.MaxLiveRoomAgentTokens {
		t.Fatalf("issued %d refused %d; want %d issued and %d refused", issued, refused,
			db.MaxLiveRoomAgentTokens, attempts-db.MaxLiveRoomAgentTokens)
	}
	if n := liveTokens(ctx, t, pool, roomID, "agent_tok_sess_race"); n != db.MaxLiveRoomAgentTokens {
		t.Fatalf("live tokens = %d; want %d", n, db.MaxLiveRoomAgentTokens)
	}
}

func TestRoomAgentTokenRepository_ConcurrentRotationsLeaveExactlyOneLiveToken(t *testing.T) {
	ctx, pool, repo, room := sessionTokenFixture(t, "rm-tok-sessions-rotrace", "agent_tok_sess_rotrace")
	roomID := room()

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := repo.Rotate(ctx, roomID, "agent_tok_sess_rotrace", 0); err != nil {
				t.Errorf("Rotate: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := liveTokens(ctx, t, pool, roomID, "agent_tok_sess_rotrace"); n != 1 {
		t.Fatalf("live tokens after 8 concurrent rotations = %d; want 1", n)
	}
}

func TestRoomAgentTokenRepository_RevokeForgetsRotatedTokensToo(t *testing.T) {
	ctx, _, repo, room := sessionTokenFixture(t, "rm-tok-sessions-revoke", "agent_tok_sess_revoke")
	roomID := room()

	old, _ := repo.Issue(ctx, roomID, "agent_tok_sess_revoke", 0)
	cur, _, _ := repo.Rotate(ctx, roomID, "agent_tok_sess_revoke", 0)
	if err := repo.Revoke(ctx, roomID, "agent_tok_sess_revoke"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	for name, plaintext := range map[string]string{"rotated": old, "current": cur} {
		h := token.HashToken(plaintext)
		if _, err := repo.ResolveByHash(ctx, h); !errors.Is(err, db.ErrAgentRoomTokenNotFound) {
			t.Fatalf("%s token must not resolve after a revoke: %v", name, err)
		}
		if rotated, _ := repo.WasRotated(ctx, h); rotated {
			t.Fatalf("a revoked agent's %s token is revoked, not 'rotated': the holder must not be told to just re-handshake", name)
		}
	}
}

func TestRoomAgentTokenRepository_RotatedTokensAreRememberedOnlyUpToABound(t *testing.T) {
	ctx, pool, repo, room := sessionTokenFixture(t, "rm-tok-sessions-bound", "agent_tok_sess_bound")
	roomID := room()

	var first string
	for i := 0; i < db.MaxRotatedRoomAgentTokens+5; i++ {
		tok, _, err := repo.Rotate(ctx, roomID, "agent_tok_sess_bound", 0)
		if err != nil {
			t.Fatalf("Rotate %d: %v", i, err)
		}
		if i == 0 {
			first = tok
		}
	}
	var rotatedRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_agent_tokens
		WHERE room_id = $1 AND agent_id = $2 AND rotated_at IS NOT NULL`, roomID, "agent_tok_sess_bound").Scan(&rotatedRows); err != nil {
		t.Fatalf("count: %v", err)
	}
	if rotatedRows > db.MaxRotatedRoomAgentTokens {
		t.Fatal(fmt.Sprintf("rotated tokens remembered = %d; want at most %d", rotatedRows, db.MaxRotatedRoomAgentTokens))
	}
	if n := liveTokens(ctx, t, pool, roomID, "agent_tok_sess_bound"); n != 1 {
		t.Fatalf("live tokens = %d; want 1", n)
	}
	if _, err := repo.ResolveByHash(ctx, token.HashToken(first)); !errors.Is(err, db.ErrAgentRoomTokenNotFound) {
		t.Fatalf("the oldest rotated token must not resolve: %v", err)
	}
}

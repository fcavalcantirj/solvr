package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// admitPresenceMember inserts an agent and gives it an active member row in the room, so
// it may be present there. Returns the agent id.
func admitPresenceMember(ctx context.Context, t *testing.T, pool *db.Pool, roomID uuid.UUID, id string) string {
	t.Helper()
	insertTestAgentForMembers(ctx, t, pool, id)
	if _, err := db.NewRoomMemberRepository(pool).Add(ctx, models.AddRoomMemberParams{
		RoomID: roomID, AgentID: id, Role: models.RoleMember,
	}); err != nil {
		t.Fatalf("admit %s: %v", id, err)
	}
	return id
}

func presentAs(ctx context.Context, t *testing.T, repo *db.AgentPresenceRepository, roomID uuid.UUID, agentID, name string) *models.AgentPresenceRecord {
	t.Helper()
	rec, err := repo.Upsert(ctx, models.UpsertAgentPresenceParams{
		RoomID: roomID, AgentID: agentID, AgentName: name, CardJSON: json.RawMessage(`{}`), TTLSeconds: 600,
	})
	if err != nil {
		t.Fatalf("Upsert(%s as %q): %v", agentID, name, err)
	}
	return rec
}

func presenceRows(ctx context.Context, t *testing.T, pool *db.Pool, roomID uuid.UUID, agentID string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_presence WHERE room_id = $1 AND agent_id = $2`,
		roomID, agentID).Scan(&n); err != nil {
		t.Fatalf("count presence: %v", err)
	}
	return n
}

func TestPresenceMembership_SchemaReferencesMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	for table, fk := range map[string]string{
		"agent_presence":    "agent_presence_membership_fkey",
		"room_agent_tokens": "room_agent_tokens_membership_fkey",
	} {
		var target string
		err := pool.QueryRow(ctx, `
			SELECT confrelid::regclass::text FROM pg_constraint
			 WHERE conrelid = $1::regclass AND conname = $2 AND contype = 'f'`, table, fk).Scan(&target)
		if err != nil || target != "room_members" {
			t.Errorf("%s: FK %s -> %q (err %v), want room_members", table, fk, target, err)
		}
	}
	var nullable string
	if err := pool.QueryRow(ctx, `SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'agent_presence' AND column_name = 'agent_id'`).Scan(&nullable); err != nil || nullable != "NO" {
		t.Errorf("agent_presence.agent_id nullable=%q err=%v, want NOT NULL column", nullable, err)
	}
}

func TestPresenceMembership_OnlyActiveMembersArePresent(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	room := createMemberTestRoom(ctx, t, pool, "pm-active-only", false)
	repo := db.NewAgentPresenceRepository(pool)
	insertTestAgentForMembers(ctx, t, pool, "agent_pm_outsider")

	_, err := repo.Upsert(ctx, models.UpsertAgentPresenceParams{
		RoomID: room.ID, AgentID: "agent_pm_outsider", AgentName: "outsider", TTLSeconds: 600,
	})
	if !errors.Is(err, db.ErrPresenceNotMember) {
		t.Fatalf("non-member Upsert err = %v, want ErrPresenceNotMember", err)
	}

	member := admitPresenceMember(ctx, t, pool, room.ID, "agent_pm_member")
	presentAs(ctx, t, repo, room.ID, member, "member-bot")
	if err := db.NewRoomMemberRepository(pool).Remove(ctx, room.ID, member); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if n := presenceRows(ctx, t, pool, room.ID, member); n != 0 {
		t.Errorf("presence rows after revocation = %d, want 0", n)
	}
	_, err = repo.Upsert(ctx, models.UpsertAgentPresenceParams{
		RoomID: room.ID, AgentID: member, AgentName: "member-bot", TTLSeconds: 600,
	})
	if !errors.Is(err, db.ErrPresenceNotMember) {
		t.Errorf("revoked member Upsert err = %v, want ErrPresenceNotMember", err)
	}
}

func TestPresenceMembership_OneRowPerMemberAndNamesAreNotStolen(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	room := createMemberTestRoom(ctx, t, pool, "pm-one-row", false)
	repo := db.NewAgentPresenceRepository(pool)
	a := admitPresenceMember(ctx, t, pool, room.ID, "agent_pm_a")
	b := admitPresenceMember(ctx, t, pool, room.ID, "agent_pm_b")

	presentAs(ctx, t, repo, room.ID, a, "alpha")
	rec := presentAs(ctx, t, repo, room.ID, a, "alpha-renamed")
	if rec.AgentID != a || rec.AgentName != "alpha-renamed" {
		t.Errorf("rename upsert = (%s,%q), want (%s,alpha-renamed)", rec.AgentID, rec.AgentName, a)
	}
	if n := presenceRows(ctx, t, pool, room.ID, a); n != 1 {
		t.Errorf("rows for one member after rename = %d, want 1", n)
	}

	_, err := repo.Upsert(ctx, models.UpsertAgentPresenceParams{
		RoomID: room.ID, AgentID: b, AgentName: "alpha-renamed", TTLSeconds: 600,
	})
	if !errors.Is(err, db.ErrPresenceNameTaken) {
		t.Fatalf("b taking a's live name err = %v, want ErrPresenceNameTaken", err)
	}

	// An EXPIRED holder no longer blocks the name.
	if _, err := pool.Exec(ctx, `UPDATE agent_presence SET last_seen = NOW() - INTERVAL '1 hour', ttl_seconds = 1
		WHERE room_id = $1 AND agent_id = $2`, room.ID, a); err != nil {
		t.Fatalf("expire a: %v", err)
	}
	if rec := presentAs(ctx, t, repo, room.ID, b, "alpha-renamed"); rec.AgentID != b {
		t.Errorf("name after expiry held by %s, want %s", rec.AgentID, b)
	}
}

func TestPresenceMembership_HeartbeatAndRemoveAreKeyedByAgent(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	room := createMemberTestRoom(ctx, t, pool, "pm-keyed", false)
	repo := db.NewAgentPresenceRepository(pool)
	a := admitPresenceMember(ctx, t, pool, room.ID, "agent_pm_ka")
	b := admitPresenceMember(ctx, t, pool, room.ID, "agent_pm_kb")
	presentAs(ctx, t, repo, room.ID, a, "shared-looking-name")

	name, err := repo.UpdateHeartbeat(ctx, room.ID, b)
	if err != nil || name != "" {
		t.Errorf("b heartbeat = (%q, %v), want (\"\", nil): b is not present", name, err)
	}
	name, err = repo.Remove(ctx, room.ID, b)
	if err != nil || name != "" {
		t.Errorf("b remove = (%q, %v), want (\"\", nil)", name, err)
	}
	if n := presenceRows(ctx, t, pool, room.ID, a); n != 1 {
		t.Fatalf("a's presence rows after b's calls = %d, want 1", n)
	}

	name, err = repo.UpdateHeartbeat(ctx, room.ID, a)
	if err != nil || name != "shared-looking-name" {
		t.Errorf("a heartbeat = (%q, %v), want its own name", name, err)
	}
	name, err = repo.Remove(ctx, room.ID, a)
	if err != nil || name != "shared-looking-name" {
		t.Errorf("a remove = (%q, %v), want its own name", name, err)
	}
	if n := presenceRows(ctx, t, pool, room.ID, a); n != 0 {
		t.Errorf("a's presence rows after remove = %d, want 0", n)
	}
}

func TestPresenceMembership_CredentialsRequireActiveMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	room := createMemberTestRoom(ctx, t, pool, "pm-credentials", false)
	tokens := db.NewRoomAgentTokenRepository(pool)
	insertTestAgentForMembers(ctx, t, pool, "agent_pm_nocred")

	if _, err := tokens.Issue(ctx, room.ID, "agent_pm_nocred", 0); err == nil {
		t.Error("Issue for a non-member succeeded, want error")
	}
	member := admitPresenceMember(ctx, t, pool, room.ID, "agent_pm_cred")
	if _, err := tokens.Issue(ctx, room.ID, member, 0); err != nil {
		t.Fatalf("Issue for a member: %v", err)
	}
	// Hard-deleting the membership row cascades to the credential through the FK.
	if _, err := pool.Exec(ctx, `DELETE FROM room_members WHERE room_id = $1 AND agent_id = $2`, room.ID, member); err != nil {
		t.Fatalf("delete membership: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_agent_tokens WHERE room_id = $1 AND agent_id = $2`,
		room.ID, member).Scan(&n); err != nil || n != 0 {
		t.Errorf("credentials after membership delete = %d (err %v), want 0", n, err)
	}
}

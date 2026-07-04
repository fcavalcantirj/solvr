package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// insertTestAgentForMembers inserts a minimal agent row for FK satisfaction.
func insertTestAgentForMembers(ctx context.Context, t *testing.T, pool *db.Pool, id string) {
	t.Helper()
	_, err := pool.Exec(ctx,
		`INSERT INTO agents (id, display_name, api_key_hash, status) VALUES ($1, $2, 'hash', 'active')
		 ON CONFLICT (id) DO NOTHING`, id, id)
	if err != nil {
		t.Fatalf("insert test agent: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, id) //nolint:errcheck
	})
}

// createMemberTestRoom creates a room via the repository and registers cleanup.
func createMemberTestRoom(ctx context.Context, t *testing.T, pool *db.Pool, slug string, private bool) *models.Room {
	t.Helper()
	repo := db.NewRoomRepository(pool)
	roomsTestCleanup(ctx, pool, slug)
	room, _, err := repo.Create(ctx, models.CreateRoomParams{
		Slug:        slug,
		DisplayName: slug,
		IsPrivate:   private,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	t.Cleanup(func() { roomsTestCleanup(context.Background(), pool, slug) })
	return room
}

func TestRoomMemberRepository_AddIsMemberIsOwner(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-members-add", true)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_owner")
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_member")

	repo := db.NewRoomMemberRepository(pool)

	// Add an owner and a plain member.
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{
		RoomID: room.ID, AgentID: "agent_rm_owner", Role: models.RoleOwner, AddedBy: "system",
	}); err != nil {
		t.Fatalf("Add owner: %v", err)
	}
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{
		RoomID: room.ID, AgentID: "agent_rm_member", Role: models.RoleMember, AddedBy: "agent_rm_owner",
	}); err != nil {
		t.Fatalf("Add member: %v", err)
	}

	// IsMember true for both.
	for _, id := range []string{"agent_rm_owner", "agent_rm_member"} {
		ok, err := repo.IsMember(ctx, room.ID, id)
		if err != nil || !ok {
			t.Fatalf("IsMember(%s) = %v, %v; want true", id, ok, err)
		}
	}
	// A non-member is not a member.
	if ok, _ := repo.IsMember(ctx, room.ID, "agent_rm_stranger"); ok {
		t.Fatalf("IsMember(stranger) = true; want false")
	}

	// IsOwner distinguishes roles.
	if ok, _ := repo.IsOwner(ctx, room.ID, "agent_rm_owner"); !ok {
		t.Fatalf("IsOwner(owner) = false; want true")
	}
	if ok, _ := repo.IsOwner(ctx, room.ID, "agent_rm_member"); ok {
		t.Fatalf("IsOwner(member) = true; want false")
	}
}

func TestRoomMemberRepository_AddIsIdempotentAndUpdatesRole(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-members-upsert", true)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_up")
	repo := db.NewRoomMemberRepository(pool)

	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_up", Role: models.RoleMember, AddedBy: "x"}); err != nil {
		t.Fatalf("Add member: %v", err)
	}
	// Re-add as owner: idempotent upsert, promotes role.
	m, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_up", Role: models.RoleOwner, AddedBy: "y"})
	if err != nil {
		t.Fatalf("Add owner (upsert): %v", err)
	}
	if m.Role != models.RoleOwner {
		t.Fatalf("role after upsert = %q; want owner", m.Role)
	}
	members, err := repo.ListByRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListByRoom: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("member count = %d; want 1 (no duplicate rows)", len(members))
	}
}

func TestRoomMemberRepository_Remove(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	room := createMemberTestRoom(ctx, t, pool, "rm-members-remove", true)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_rem")
	repo := db.NewRoomMemberRepository(pool)

	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_rem", Role: models.RoleMember, AddedBy: "x"}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := repo.Remove(ctx, room.ID, "agent_rm_rem"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if ok, _ := repo.IsMember(ctx, room.ID, "agent_rm_rem"); ok {
		t.Fatalf("still a member after Remove")
	}
	// Removing a non-member returns ErrRoomMemberNotFound.
	if err := repo.Remove(ctx, room.ID, "agent_rm_rem"); err != db.ErrRoomMemberNotFound {
		t.Fatalf("Remove(non-member) err = %v; want ErrRoomMemberNotFound", err)
	}
}

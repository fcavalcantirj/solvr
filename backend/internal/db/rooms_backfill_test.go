package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// TestRoomRepository_BackfillOwnerFromMembership verifies the on-claim owner backfill:
// it sets owner_id only on NULL-owner rooms where the agent holds an 'owner' membership,
// leaves member-only and already-owned rooms untouched, and is idempotent.
func TestRoomRepository_BackfillOwnerFromMembership(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()

	repo := db.NewRoomRepository(pool)
	memberRepo := db.NewRoomMemberRepository(pool)

	// Real user for the rooms.owner_id FK.
	humanID := uuid.New()
	stamp := time.Now().Format("150405.000")
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, display_name, email, username, referral_code)
		VALUES ($1, 'BF Owner', $2, $3, $4) ON CONFLICT (id) DO NOTHING
	`, humanID, "bf-"+humanID.String()[:8]+"@test.com", "bf"+stamp[:6], stamp[:8])
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", humanID) }) //nolint:errcheck

	agentID := "agent_bf_owner"
	insertTestAgentForMembers(ctx, t, pool, agentID)

	// Room 1: agent is OWNER, owner_id NULL -> should be backfilled.
	ownRoom := createMemberTestRoom(ctx, t, pool, "testroombf-own", true)
	if _, err := memberRepo.Add(ctx, models.AddRoomMemberParams{
		RoomID: ownRoom.ID, AgentID: agentID, Role: models.RoleOwner, AddedBy: "system",
	}); err != nil {
		t.Fatalf("add owner membership: %v", err)
	}

	// Room 2: agent is only a MEMBER, owner_id NULL -> must NOT be backfilled.
	memRoom := createMemberTestRoom(ctx, t, pool, "testroombf-mem", true)
	if _, err := memberRepo.Add(ctx, models.AddRoomMemberParams{
		RoomID: memRoom.ID, AgentID: agentID, Role: models.RoleMember, AddedBy: "system",
	}); err != nil {
		t.Fatalf("add member membership: %v", err)
	}

	// Backfill: exactly one room (the owned, NULL-owner one) is updated.
	n, err := repo.BackfillOwnerFromMembership(ctx, agentID, humanID.String())
	if err != nil {
		t.Fatalf("BackfillOwnerFromMembership: %v", err)
	}
	if n != 1 {
		t.Fatalf("rows affected = %d, want 1", n)
	}

	got, err := repo.GetBySlug(ctx, ownRoom.Slug)
	if err != nil {
		t.Fatalf("get owned room: %v", err)
	}
	if got.OwnerID == nil || *got.OwnerID != humanID {
		t.Errorf("owned room owner_id = %v, want %v", got.OwnerID, humanID)
	}

	gotMem, err := repo.GetBySlug(ctx, memRoom.Slug)
	if err != nil {
		t.Fatalf("get member room: %v", err)
	}
	if gotMem.OwnerID != nil {
		t.Errorf("member-only room owner_id = %v, want nil (must not be backfilled)", gotMem.OwnerID)
	}

	// Idempotent: nothing left to update.
	n2, err := repo.BackfillOwnerFromMembership(ctx, agentID, humanID.String())
	if err != nil {
		t.Fatalf("BackfillOwnerFromMembership (2nd): %v", err)
	}
	if n2 != 0 {
		t.Errorf("rows affected on 2nd run = %d, want 0 (idempotent)", n2)
	}
}

package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Tests for migration 000100: the account lifecycle leaves every room in a documented
// state. Agent unlinking (human_id -> NULL, SPEC Part 20.2) is allowed while a direct
// re-claim X -> Y stays forbidden; a soft-deleted human leaves its rooms, and a room
// whose only live owner it was is archived exactly like a hard-deleted owner's room.

// userMembership reports whether the human holds a row in the room and whether it is active.
func userMembership(ctx context.Context, t *testing.T, pool *db.Pool, roomID, userID uuid.UUID) (exists, active bool) {
	t.Helper()
	err := pool.QueryRow(ctx,
		`SELECT COUNT(*) > 0, COALESCE(bool_or(revoked_at IS NULL), false)
		 FROM room_members WHERE room_id = $1 AND user_id = $2`, roomID, userID).Scan(&exists, &active)
	if err != nil {
		t.Fatalf("query membership: %v", err)
	}
	return exists, active
}

func softDeleteUser(ctx context.Context, t *testing.T, pool *db.Pool, userID uuid.UUID) {
	t.Helper()
	if _, err := pool.Exec(ctx, `UPDATE users SET deleted_at = NOW() WHERE id = $1`, userID); err != nil {
		t.Fatalf("soft delete user: %v", err)
	}
}

func TestRoomMembersLifecycle_UnlinkAllowedReclaimForbidden(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	first := insertAuthorityUser(ctx, t, pool, "unlinka")
	second := insertAuthorityUser(ctx, t, pool, "unlinkb")
	linkAgent(ctx, t, pool, "agent_rm_life_unlink", &first)

	_, err := pool.Exec(ctx, `UPDATE agents SET human_id = $2 WHERE id = $1`, "agent_rm_life_unlink", second)
	if err == nil || !strings.Contains(err.Error(), "agent_already_claimed") {
		t.Fatalf("direct re-claim to another human: err = %v, want agent_already_claimed", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agents SET human_id = NULL WHERE id = $1`, "agent_rm_life_unlink"); err != nil {
		t.Fatalf("unlink (DELETE /v1/me unclaim) refused: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agents SET human_id = $2 WHERE id = $1`, "agent_rm_life_unlink", second); err != nil {
		t.Fatalf("claim of an unclaimed agent by another human refused: %v", err)
	}
}

// Unlinking keeps the agent's own direct memberships and ends only family-derived ones.
func TestRoomMembersLifecycle_UnlinkKeepsDirectMemberships(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	human := insertAuthorityUser(ctx, t, pool, "unlinkdir")
	linkAgent(ctx, t, pool, "agent_rm_life_direct", &human)
	room := createAuthorityRoom(ctx, t, pool, "rm-life-direct", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_life_direct",
	})
	if _, err := pool.Exec(ctx, `UPDATE agents SET human_id = NULL WHERE id = $1`, "agent_rm_life_direct"); err != nil {
		t.Fatalf("unlink: %v", err)
	}
	owner, err := db.NewRoomMemberRepository(pool).IsOwner(ctx, room.ID, "agent_rm_life_direct")
	if err != nil || !owner {
		t.Fatalf("unlinked agent lost its own owner membership: owner=%v err=%v", owner, err)
	}
	if roomArchived(ctx, t, pool, room.ID) {
		t.Fatal("unlinking archived a room that still has its owner")
	}
}

func TestRoomMembersLifecycle_SoftDeletedSoleOwnerArchivesRoom(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	user := insertAuthorityUser(ctx, t, pool, "softsole")
	room := createAuthorityRoom(ctx, t, pool, "rm-life-soft-sole", models.CreateRoomParams{IsPrivate: true, OwnerID: user})

	softDeleteUser(ctx, t, pool, user)

	if !roomArchived(ctx, t, pool, room.ID) {
		t.Fatal("room whose only owner soft-deleted the account is not archived")
	}
	// The owner row is kept so an admin account recovery restores ownership.
	if exists, active := userMembership(ctx, t, pool, room.ID, user); !exists || !active {
		t.Fatalf("sole owner row: exists=%v active=%v, want kept active", exists, active)
	}
}

func TestRoomMembersLifecycle_SoftDeletedCoOwnerAndMemberLeave(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_life_co")
	user := insertAuthorityUser(ctx, t, pool, "softco")
	shared := createAuthorityRoom(ctx, t, pool, "rm-life-soft-shared", models.CreateRoomParams{
		IsPrivate: true, OwnerID: user, CreatorAgentID: "agent_rm_life_co",
	})
	joined := createAuthorityRoom(ctx, t, pool, "rm-life-soft-joined", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_life_co",
	})
	if _, err := pool.Exec(ctx,
		`INSERT INTO room_members (room_id, user_id, role, added_by) VALUES ($1, $2, 'member', 'test')`,
		joined.ID, user); err != nil {
		t.Fatalf("add human member: %v", err)
	}

	softDeleteUser(ctx, t, pool, user)

	for _, r := range []*models.Room{shared, joined} {
		if roomArchived(ctx, t, pool, r.ID) {
			t.Fatalf("room %s keeps a live owner but was archived", r.Slug)
		}
		if exists, active := userMembership(ctx, t, pool, r.ID, user); !exists || active {
			t.Fatalf("room %s: deleted human membership exists=%v active=%v, want revoked", r.Slug, exists, active)
		}
	}
	owner, err := db.NewRoomMemberRepository(pool).IsOwner(ctx, shared.ID, "agent_rm_life_co")
	if err != nil || !owner {
		t.Fatalf("remaining agent owner: owner=%v err=%v", owner, err)
	}
}

// An owner whose own account is soft-deleted does not count as a live owner.
func TestRoomMembersLifecycle_SoftDeletedOwnersDoNotKeepRoomLive(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_life_gone")
	user := insertAuthorityUser(ctx, t, pool, "softlast")
	room := createAuthorityRoom(ctx, t, pool, "rm-life-soft-last", models.CreateRoomParams{
		IsPrivate: true, OwnerID: user, CreatorAgentID: "agent_rm_life_gone",
	})
	if _, err := pool.Exec(ctx, `UPDATE agents SET deleted_at = NOW() WHERE id = $1`, "agent_rm_life_gone"); err != nil {
		t.Fatalf("soft delete agent: %v", err)
	}

	softDeleteUser(ctx, t, pool, user)

	if !roomArchived(ctx, t, pool, room.ID) {
		t.Fatal("room left with only soft-deleted owners is not archived")
	}
}

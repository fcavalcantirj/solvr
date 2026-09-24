package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/google/uuid"
)

// Tests for migration 000096: family access (an agent whose linked human owns the room)
// is derived from the human's ACTIVE owner membership plus the agent's CURRENT link, and
// memberships materialized through family access end when that justification ends.

// linkAgent inserts a minimal agent and links it to the given human (nil = unclaimed).
func linkAgent(ctx context.Context, t *testing.T, pool *db.Pool, agentID string, human *uuid.UUID) {
	t.Helper()
	insertTestAgentForMembers(ctx, t, pool, agentID)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, agentID) }) //nolint:errcheck
	if human == nil {
		return
	}
	if _, err := pool.Exec(ctx, `UPDATE agents SET human_id = $2 WHERE id = $1`, agentID, *human); err != nil {
		t.Fatalf("link agent: %v", err)
	}
}

// forceUnlink clears agents.human_id. Migration 000018's prevent_agent_reclaim refused
// that change until 000100 allowed unlinking; lifting the trigger for this one committed
// transaction is kept so these tests do not depend on that migration.
func forceUnlink(ctx context.Context, t *testing.T, pool *db.Pool, agentIDs ...string) {
	t.Helper()
	err := pool.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `ALTER TABLE agents DISABLE TRIGGER trigger_prevent_agent_reclaim`); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE agents SET human_id = NULL WHERE id = ANY($1)`, agentIDs); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `ALTER TABLE agents ENABLE TRIGGER trigger_prevent_agent_reclaim`)
		return err
	})
	if err != nil {
		t.Fatalf("force unlink: %v", err)
	}
}

func mustBool(t *testing.T, what string, got bool, err error, want bool) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
	if got != want {
		t.Fatalf("%s = %v; want %v", what, got, want)
	}
}

func TestRoomMembersFamily_AccessFollowsHumanOwnerMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "famown")
	other := insertAuthorityUser(ctx, t, pool, "famoth")
	room := createAuthorityRoom(ctx, t, pool, "rm-fam-access", models.CreateRoomParams{IsPrivate: true, OwnerID: owner})
	linkAgent(ctx, t, pool, "agent_rm_fam_sib", &owner)
	linkAgent(ctx, t, pool, "agent_rm_fam_foreign", &other)
	linkAgent(ctx, t, pool, "agent_rm_fam_unclaimed", nil)
	repo := db.NewRoomMemberRepository(pool)

	ok, err := repo.IsFamilyOwner(ctx, room.ID, "agent_rm_fam_sib")
	mustBool(t, "sibling IsFamilyOwner", ok, err, true)
	ok, err = repo.IsFamilyOwner(ctx, room.ID, "agent_rm_fam_foreign")
	mustBool(t, "foreign IsFamilyOwner", ok, err, false)
	ok, err = repo.IsFamilyOwner(ctx, room.ID, "agent_rm_fam_unclaimed")
	mustBool(t, "unclaimed IsFamilyOwner", ok, err, false)

	ok, err = repo.IsUserOwner(ctx, room.ID, owner.String())
	mustBool(t, "owner IsUserOwner", ok, err, true)
	ok, err = repo.IsUserMember(ctx, room.ID, owner.String())
	mustBool(t, "owner IsUserMember", ok, err, true)
	ok, err = repo.IsUserOwner(ctx, room.ID, other.String())
	mustBool(t, "other IsUserOwner", ok, err, false)
	ok, err = repo.IsUserMember(ctx, room.ID, "not-a-uuid")
	mustBool(t, "malformed IsUserMember", ok, err, false)

	// Membership, not rooms.owner_id, is the authority: with a second owner in place,
	// revoke the human's owner row while owner_id still names them.
	linkAgent(ctx, t, pool, "agent_rm_fam_heir", nil)
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_fam_heir", Role: models.RoleOwner, AddedBy: "test"}); err != nil {
		t.Fatalf("add heir: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE room_members SET revoked_at = NOW() WHERE room_id = $1 AND user_id = $2`, room.ID, owner); err != nil {
		t.Fatalf("revoke human owner: %v", err)
	}
	ok, err = repo.IsUserOwner(ctx, room.ID, owner.String())
	mustBool(t, "revoked human IsUserOwner", ok, err, false)
	ok, err = repo.IsFamilyOwner(ctx, room.ID, "agent_rm_fam_sib")
	mustBool(t, "sibling after human revoked", ok, err, false)
}

func TestRoomMembersFamily_UnlinkEndsFamilyMaterializedMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "famunl")
	room := createAuthorityRoom(ctx, t, pool, "rm-fam-unlink", models.CreateRoomParams{IsPrivate: true, OwnerID: owner})
	linkAgent(ctx, t, pool, "agent_rm_fam_unl", &owner)
	repo := db.NewRoomMemberRepository(pool)
	tokens := db.NewRoomAgentTokenRepository(pool)

	m, err := repo.AddFamily(ctx, room.ID, "agent_rm_fam_unl")
	if err != nil {
		t.Fatalf("AddFamily: %v", err)
	}
	if m.Role != models.RoleMember || m.AccessSource != models.AccessSourceFamily {
		t.Fatalf("materialized membership = %+v; want member via family", m)
	}
	tok, err := tokens.Issue(ctx, room.ID, "agent_rm_fam_unl", 0)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	forceUnlink(ctx, t, pool, "agent_rm_fam_unl")
	ok, err := repo.IsMember(ctx, room.ID, "agent_rm_fam_unl")
	mustBool(t, "unlinked IsMember", ok, err, false)
	ok, err = repo.IsFamilyOwner(ctx, room.ID, "agent_rm_fam_unl")
	mustBool(t, "unlinked IsFamilyOwner", ok, err, false)
	if _, err := tokens.ResolveByHash(ctx, token.HashToken(tok)); err == nil {
		t.Fatal("family-derived room token must die with the link")
	}
}

func TestRoomMembersFamily_DirectMembershipSurvivesUnlink(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "famdir")
	room := createAuthorityRoom(ctx, t, pool, "rm-fam-direct", models.CreateRoomParams{IsPrivate: true, OwnerID: owner})
	linkAgent(ctx, t, pool, "agent_rm_fam_dir", &owner)
	linkAgent(ctx, t, pool, "agent_rm_fam_promoted", &owner)
	repo := db.NewRoomMemberRepository(pool)

	// Explicitly allowlisted by the owner: a direct grant, independent of the link.
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_fam_dir", AddedBy: owner.String()}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Family-materialized, then explicitly added: the explicit grant wins.
	if _, err := repo.AddFamily(ctx, room.ID, "agent_rm_fam_promoted"); err != nil {
		t.Fatalf("AddFamily: %v", err)
	}
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_fam_promoted", AddedBy: owner.String()}); err != nil {
		t.Fatalf("Add after AddFamily: %v", err)
	}
	// AddFamily never downgrades an existing direct grant.
	m, err := repo.AddFamily(ctx, room.ID, "agent_rm_fam_dir")
	if err != nil {
		t.Fatalf("AddFamily on direct member: %v", err)
	}
	if m.AccessSource != models.AccessSourceDirect {
		t.Fatalf("AddFamily downgraded a direct membership: %+v", m)
	}

	forceUnlink(ctx, t, pool, "agent_rm_fam_dir", "agent_rm_fam_promoted")
	ok, err := repo.IsMember(ctx, room.ID, "agent_rm_fam_dir")
	mustBool(t, "direct member after unlink", ok, err, true)
	ok, err = repo.IsMember(ctx, room.ID, "agent_rm_fam_promoted")
	mustBool(t, "explicitly added member after unlink", ok, err, true)
}

func TestRoomMembersFamily_OwnerLeavingEndsFamilyMemberships(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "famlv")
	room := createAuthorityRoom(ctx, t, pool, "rm-fam-leave", models.CreateRoomParams{IsPrivate: true, OwnerID: owner})
	linkAgent(ctx, t, pool, "agent_rm_fam_lv", &owner)
	linkAgent(ctx, t, pool, "agent_rm_fam_lvheir", nil)
	repo := db.NewRoomMemberRepository(pool)

	if _, err := repo.AddFamily(ctx, room.ID, "agent_rm_fam_lv"); err != nil {
		t.Fatalf("AddFamily: %v", err)
	}
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_fam_lvheir", Role: models.RoleOwner, AddedBy: "test"}); err != nil {
		t.Fatalf("add heir: %v", err)
	}
	// Demoting the human owner (transfer) ends the family grant it justified.
	if _, err := pool.Exec(ctx, `UPDATE room_members SET role = 'member' WHERE room_id = $1 AND user_id = $2`, room.ID, owner); err != nil {
		t.Fatalf("demote human owner: %v", err)
	}
	ok, err := repo.IsMember(ctx, room.ID, "agent_rm_fam_lv")
	mustBool(t, "family member after owner demoted", ok, err, false)

	// Readmission is explicit: an owner Add restores it as a direct member.
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_fam_lv", AddedBy: "agent_rm_fam_lvheir"}); err != nil {
		t.Fatalf("readmit: %v", err)
	}
	ok, err = repo.IsMember(ctx, room.ID, "agent_rm_fam_lv")
	mustBool(t, "readmitted member", ok, err, true)
}

func TestRoomMembersFamily_HumanAccountDeletionEndsFamilyMemberships(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "famdel")
	room := createAuthorityRoom(ctx, t, pool, "rm-fam-del", models.CreateRoomParams{IsPrivate: true, OwnerID: owner})
	linkAgent(ctx, t, pool, "agent_rm_fam_del", &owner)
	linkAgent(ctx, t, pool, "agent_rm_fam_delheir", nil)
	repo := db.NewRoomMemberRepository(pool)
	if _, err := repo.AddFamily(ctx, room.ID, "agent_rm_fam_del"); err != nil {
		t.Fatalf("AddFamily: %v", err)
	}
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{RoomID: room.ID, AgentID: "agent_rm_fam_delheir", Role: models.RoleOwner, AddedBy: "test"}); err != nil {
		t.Fatalf("add heir: %v", err)
	}

	// DELETE /v1/me soft-deletes the account (users.deleted_at); the agent's link row
	// stays, so a deleted human's family grants must end on their own.
	if err := db.NewUserRepository(pool).Delete(ctx, owner.String()); err != nil {
		t.Fatalf("soft-delete user: %v", err)
	}
	ok, err := repo.IsFamilyOwner(ctx, room.ID, "agent_rm_fam_del")
	mustBool(t, "family of a deleted human", ok, err, false)
	ok, err = repo.IsMember(ctx, room.ID, "agent_rm_fam_del")
	mustBool(t, "family member after owner account deleted", ok, err, false)
	ok, err = repo.IsOwner(ctx, room.ID, "agent_rm_fam_delheir")
	mustBool(t, "heir still owns", ok, err, true)
	if _, err := repo.Get(ctx, room.ID, "agent_rm_fam_del"); !errors.Is(err, db.ErrRoomMemberNotFound) {
		t.Fatalf("Get after family revoke = %v; want ErrRoomMemberNotFound", err)
	}
}

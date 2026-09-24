package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Tests for migration 000095: room_members is the single membership authority for
// humans (user_id) and agents (agent_id), with in-place revocation, a final-owner
// guard, and per-agent credentials bound to the active membership.

func openAuthorityPool(t *testing.T) (context.Context, *db.Pool) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool
}

// insertAuthorityUser inserts a minimal human account and registers cleanup.
func insertAuthorityUser(ctx context.Context, t *testing.T, pool *db.Pool, tag string) uuid.UUID {
	t.Helper()
	suffix := time.Now().Format("150405.000000")
	var id uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code)
		 VALUES ($1, $1, $2, 'email', $2, $3) RETURNING id`,
		"rmauth"+tag+suffix[len(suffix)-6:], "rmauth-"+tag+"-"+suffix+"@example.test",
		uuid.NewString()[:8],
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) }) //nolint:errcheck
	return id
}

func createAuthorityRoom(ctx context.Context, t *testing.T, pool *db.Pool, slug string, params models.CreateRoomParams) *models.Room {
	t.Helper()
	roomsTestCleanup(ctx, pool, slug)
	params.Slug = slug
	params.DisplayName = slug
	room, _, err := db.NewRoomRepository(pool).Create(ctx, params)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	t.Cleanup(func() { roomsTestCleanup(context.Background(), pool, slug) })
	return room
}

func activeHumanOwner(ctx context.Context, t *testing.T, pool *db.Pool, roomID, userID uuid.UUID) bool {
	t.Helper()
	var ok bool
	err := pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM room_members WHERE room_id = $1 AND user_id = $2
		   AND role = 'owner' AND agent_id IS NULL AND revoked_at IS NULL)`, roomID, userID).Scan(&ok)
	if err != nil {
		t.Fatalf("query human owner: %v", err)
	}
	return ok
}

func TestRoomMembersAuthority_ExactlyOneActor(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	room := createAuthorityRoom(ctx, t, pool, "rm-auth-one-actor", models.CreateRoomParams{IsPrivate: true})
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_actor")
	user := insertAuthorityUser(ctx, t, pool, "actor")

	// Neither actor set: rejected.
	if _, err := pool.Exec(ctx,
		`INSERT INTO room_members (room_id, role, added_by) VALUES ($1, 'member', 'test')`, room.ID); err == nil {
		t.Fatal("membership with no actor was accepted; want exactly-one-actor violation")
	}
	// Both actors set: rejected.
	if _, err := pool.Exec(ctx,
		`INSERT INTO room_members (room_id, agent_id, user_id, role, added_by) VALUES ($1, $2, $3, 'member', 'test')`,
		room.ID, "agent_rm_auth_actor", user); err == nil {
		t.Fatal("membership with two actors was accepted; want exactly-one-actor violation")
	}
	// A human member with an optional display role is a valid row.
	if _, err := pool.Exec(ctx,
		`INSERT INTO room_members (room_id, user_id, role, display_role, added_by) VALUES ($1, $2, 'member', 'reviewer', 'test')`,
		room.ID, user); err != nil {
		t.Fatalf("human membership rejected: %v", err)
	}
	// Unique active membership per human and room.
	if _, err := pool.Exec(ctx,
		`INSERT INTO room_members (room_id, user_id, role, added_by) VALUES ($1, $2, 'member', 'test')`,
		room.ID, user); err == nil {
		t.Fatal("duplicate human membership accepted; want unique violation")
	}
}

func TestRoomMembersAuthority_HumanOwnerEstablishedAtCreation(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	user := insertAuthorityUser(ctx, t, pool, "creator")
	room := createAuthorityRoom(ctx, t, pool, "rm-auth-human-create", models.CreateRoomParams{IsPrivate: true, OwnerID: user})

	if !activeHumanOwner(ctx, t, pool, room.ID, user) {
		t.Fatal("room created by a human has no active human owner membership")
	}
}

func TestRoomMembersAuthority_ClaimLinksHumanOwnerMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_claimed")
	room := createAuthorityRoom(ctx, t, pool, "rm-auth-claim", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_auth_claimed",
	})
	user := insertAuthorityUser(ctx, t, pool, "claimer")

	n, err := db.NewRoomRepository(pool).BackfillOwnerFromMembership(ctx, "agent_rm_auth_claimed", user.String())
	if err != nil || n != 1 {
		t.Fatalf("BackfillOwnerFromMembership = %d, %v; want 1, nil", n, err)
	}
	if !activeHumanOwner(ctx, t, pool, room.ID, user) {
		t.Fatal("claiming the owner agent did not give its human an owner membership")
	}
	// The agent's own owner membership is untouched.
	if ok, _ := db.NewRoomMemberRepository(pool).IsOwner(ctx, room.ID, "agent_rm_auth_claimed"); !ok {
		t.Fatal("agent lost its owner membership after being linked to a human")
	}
}

func TestRoomMembersAuthority_RevokeAndReadmit(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_owner")
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_exec")
	room := createAuthorityRoom(ctx, t, pool, "rm-auth-revoke", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_auth_owner",
	})
	repo := db.NewRoomMemberRepository(pool)
	tokens := db.NewRoomAgentTokenRepository(pool)

	if _, err := repo.Add(ctx, models.AddRoomMemberParams{
		RoomID: room.ID, AgentID: "agent_rm_auth_exec", Role: models.RoleMember, AddedBy: "agent_rm_auth_owner",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if _, err := tokens.Issue(ctx, room.ID, "agent_rm_auth_exec", 0); err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if err := repo.Remove(ctx, room.ID, "agent_rm_auth_exec"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	// Revocation is recorded, not erased.
	var revoked bool
	if err := pool.QueryRow(ctx,
		`SELECT revoked_at IS NOT NULL FROM room_members WHERE room_id = $1 AND agent_id = $2`,
		room.ID, "agent_rm_auth_exec").Scan(&revoked); err != nil || !revoked {
		t.Fatalf("revoked membership row: revoked=%v err=%v; want a row with revoked_at set", revoked, err)
	}
	if ok, _ := repo.IsMember(ctx, room.ID, "agent_rm_auth_exec"); ok {
		t.Fatal("revoked agent still IsMember")
	}
	if _, err := repo.Get(ctx, room.ID, "agent_rm_auth_exec"); !errors.Is(err, db.ErrRoomMemberNotFound) {
		t.Fatalf("Get(revoked) err = %v; want ErrRoomMemberNotFound", err)
	}
	members, err := repo.ListByRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListByRoom: %v", err)
	}
	for _, m := range members {
		if m.AgentID == "agent_rm_auth_exec" {
			t.Fatal("ListByRoom returned a revoked member")
		}
	}
	// The per-agent credential is bound to the membership: revocation removes it.
	var tokenRows int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_agent_tokens WHERE room_id = $1 AND agent_id = $2`,
		room.ID, "agent_rm_auth_exec").Scan(&tokenRows); err != nil || tokenRows != 0 {
		t.Fatalf("room_agent_tokens rows after revocation = %d (err %v); want 0", tokenRows, err)
	}
	// A second removal of a revoked member is NotFound.
	if err := repo.Remove(ctx, room.ID, "agent_rm_auth_exec"); !errors.Is(err, db.ErrRoomMemberNotFound) {
		t.Fatalf("second Remove err = %v; want ErrRoomMemberNotFound", err)
	}

	// Explicit readmission restores the active membership.
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{
		RoomID: room.ID, AgentID: "agent_rm_auth_exec", Role: models.RoleMember, AddedBy: "agent_rm_auth_owner",
	}); err != nil {
		t.Fatalf("readmit Add: %v", err)
	}
	if ok, _ := repo.IsMember(ctx, room.ID, "agent_rm_auth_exec"); !ok {
		t.Fatal("readmitted agent is not a member")
	}
}

func TestRoomMembersAuthority_FinalOwnerGuard(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_sole")
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_heir")
	room := createAuthorityRoom(ctx, t, pool, "rm-auth-final-owner", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_auth_sole",
	})
	repo := db.NewRoomMemberRepository(pool)

	if err := repo.Remove(ctx, room.ID, "agent_rm_auth_sole"); !errors.Is(err, db.ErrLastRoomOwner) {
		t.Fatalf("removing the final owner err = %v; want ErrLastRoomOwner", err)
	}
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{
		RoomID: room.ID, AgentID: "agent_rm_auth_sole", Role: models.RoleMember, AddedBy: "agent_rm_auth_sole",
	}); !errors.Is(err, db.ErrLastRoomOwner) {
		t.Fatalf("demoting the final owner err = %v; want ErrLastRoomOwner", err)
	}
	if ok, _ := repo.IsOwner(ctx, room.ID, "agent_rm_auth_sole"); !ok {
		t.Fatal("final owner lost ownership after a refused removal")
	}

	// Ownership transfer: add the heir as owner, then the previous owner may leave.
	if _, err := repo.Add(ctx, models.AddRoomMemberParams{
		RoomID: room.ID, AgentID: "agent_rm_auth_heir", Role: models.RoleOwner, AddedBy: "agent_rm_auth_sole",
	}); err != nil {
		t.Fatalf("add heir owner: %v", err)
	}
	if err := repo.Remove(ctx, room.ID, "agent_rm_auth_sole"); err != nil {
		t.Fatalf("remove previous owner after transfer: %v", err)
	}
	if ok, _ := repo.IsOwner(ctx, room.ID, "agent_rm_auth_heir"); !ok {
		t.Fatal("heir is not the owner after transfer")
	}
}

func TestRoomMembersAuthority_DeletedRoomReleasesFinalOwner(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_deleter")
	room := createAuthorityRoom(ctx, t, pool, "rm-auth-deleted-room", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_auth_deleter",
	})
	if err := db.NewRoomRepository(pool).SoftDelete(ctx, room.ID); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if err := db.NewRoomMemberRepository(pool).Remove(ctx, room.ID, "agent_rm_auth_deleter"); err != nil {
		t.Fatalf("removing the owner of a deleted room: %v; want nil", err)
	}
	// Hard-deleting a room cascades its memberships without tripping the guard.
	if _, err := pool.Exec(ctx, `DELETE FROM rooms WHERE id = $1`, room.ID); err != nil {
		t.Fatalf("hard delete room: %v", err)
	}
}

func roomArchived(ctx context.Context, t *testing.T, pool *db.Pool, roomID uuid.UUID) bool {
	t.Helper()
	var archived bool
	if err := pool.QueryRow(ctx, `SELECT archived_at IS NOT NULL FROM rooms WHERE id = $1`, roomID).Scan(&archived); err != nil {
		t.Fatalf("query archived_at: %v", err)
	}
	return archived
}

func TestRoomMembersAuthority_AccountDeletionArchivesOwnerlessRoom(t *testing.T) {
	ctx, pool := openAuthorityPool(t)

	// Sole agent owner hard-deleted: the room is archived, not left unmanageable.
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_gone")
	agentRoom := createAuthorityRoom(ctx, t, pool, "rm-auth-agent-gone", models.CreateRoomParams{
		IsPrivate: true, CreatorAgentID: "agent_rm_auth_gone",
	})
	if _, err := pool.Exec(ctx, `DELETE FROM agents WHERE id = $1`, "agent_rm_auth_gone"); err != nil {
		t.Fatalf("hard delete agent: %v", err)
	}
	if !roomArchived(ctx, t, pool, agentRoom.ID) {
		t.Fatal("room whose only owner agent was deleted is not archived")
	}

	// Sole human owner hard-deleted: same documented archived state.
	user := insertAuthorityUser(ctx, t, pool, "gone")
	humanRoom := createAuthorityRoom(ctx, t, pool, "rm-auth-human-gone", models.CreateRoomParams{IsPrivate: true, OwnerID: user})
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user); err != nil {
		t.Fatalf("hard delete user: %v", err)
	}
	if !roomArchived(ctx, t, pool, humanRoom.ID) {
		t.Fatal("room whose only human owner was deleted is not archived")
	}

	// A room that keeps another owner stays active when one owner account is deleted.
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_auth_stays")
	other := insertAuthorityUser(ctx, t, pool, "leaves")
	shared := createAuthorityRoom(ctx, t, pool, "rm-auth-shared-owner", models.CreateRoomParams{
		IsPrivate: true, OwnerID: other, CreatorAgentID: "agent_rm_auth_stays",
	})
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, other); err != nil {
		t.Fatalf("hard delete co-owner: %v", err)
	}
	if roomArchived(ctx, t, pool, shared.ID) {
		t.Fatal("room with a remaining owner was archived")
	}
}

// Before/after capability matrix: every room whose legacy owner_id names a human has
// exactly that human as an active owner in room_members, and every agent owner
// membership survives the schema change. Runs over ALL rooms in the database.
// rooms.owner_id is retired by 000097, so the legacy column is rebuilt by that
// migration's down file inside a rolled-back transaction and compared from there.
func TestRoomMembersAuthority_OwnershipMatrixMatchesLegacyOwner(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background()) //nolint:errcheck
	runMigrationFile(ctx, t, tx, "000097_retire_rooms_owner_id.down.sql")
	var missingHuman, strayHuman int
	if err := tx.QueryRow(ctx, `
		SELECT
		  (SELECT COUNT(*) FROM rooms r WHERE r.owner_id IS NOT NULL AND NOT EXISTS (
		     SELECT 1 FROM room_members m WHERE m.room_id = r.id AND m.user_id = r.owner_id
		       AND m.role = 'owner' AND m.revoked_at IS NULL)),
		  (SELECT COUNT(*) FROM room_members m JOIN rooms r ON r.id = m.room_id
		     WHERE m.user_id IS NOT NULL AND m.role = 'owner' AND m.revoked_at IS NULL
		       AND r.owner_id IS DISTINCT FROM m.user_id)`).Scan(&missingHuman, &strayHuman); err != nil {
		t.Fatalf("matrix query: %v", err)
	}
	if missingHuman != 0 || strayHuman != 0 {
		t.Fatalf("human ownership drift: %d rooms missing their owner_id membership, %d human owner memberships without a matching owner_id", missingHuman, strayHuman)
	}
}

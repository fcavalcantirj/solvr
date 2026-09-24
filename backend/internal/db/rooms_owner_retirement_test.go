package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Tests for migration 000097: rooms.owner_id and its sync trigger are retired.
// room_members is the only ownership store; a room's owner_id in API responses is
// derived from its earliest active human owner membership.

func TestRoomsOwnerRetired_ColumnTriggerAndIndexGone(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	var column, trigger, index bool
	if err := pool.QueryRow(ctx, `
		SELECT
		  EXISTS(SELECT 1 FROM information_schema.columns
		         WHERE table_schema = 'public' AND table_name = 'rooms' AND column_name = 'owner_id'),
		  EXISTS(SELECT 1 FROM pg_trigger WHERE tgname = 'rooms_sync_human_owner'),
		  EXISTS(SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'idx_rooms_owner_id')`,
	).Scan(&column, &trigger, &index); err != nil {
		t.Fatalf("schema query: %v", err)
	}
	if column || trigger || index {
		t.Fatalf("legacy ownership state still present: owner_id column=%v, sync trigger=%v, index=%v", column, trigger, index)
	}
}

func TestRoomsOwnerRetired_CreateEstablishesHumanOwnerMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "retcreate")
	room := createAuthorityRoom(ctx, t, pool, "rm-ret-create", models.CreateRoomParams{IsPrivate: true, OwnerID: owner})

	if room.OwnerID == nil || *room.OwnerID != owner {
		t.Fatalf("Create owner_id = %v, want %v", room.OwnerID, owner)
	}
	if !activeHumanOwner(ctx, t, pool, room.ID, owner) {
		t.Fatal("Create did not establish the human owner membership")
	}
	got, err := db.NewRoomRepository(pool).GetBySlug(ctx, room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.OwnerID == nil || *got.OwnerID != owner {
		t.Fatalf("GetBySlug owner_id = %v, want %v", got.OwnerID, owner)
	}

	// An agent-created room without a human has no derived owner_id.
	insertTestAgentForMembers(ctx, t, pool, "agent_rm_ret_creator")
	agentRoom := createAuthorityRoom(ctx, t, pool, "rm-ret-agent", models.CreateRoomParams{CreatorAgentID: "agent_rm_ret_creator"})
	if agentRoom.OwnerID != nil {
		t.Fatalf("agent-created room owner_id = %v, want nil", agentRoom.OwnerID)
	}
}

// The derived owner follows the membership authority: after a human-to-human transfer
// every read path (GetBySlug, ListByOwner, public discovery) reports the new owner.
func TestRoomsOwnerRetired_DerivedOwnerFollowsMembershipTransfer(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	first := insertAuthorityUser(ctx, t, pool, "retfirst")
	heir := insertAuthorityUser(ctx, t, pool, "retheir")
	room := createAuthorityRoom(ctx, t, pool, "rm-ret-transfer", models.CreateRoomParams{OwnerID: first})
	repo := db.NewRoomRepository(pool)

	// Transfer in one transaction: promote the heir, revoke the first owner.
	if err := pool.WithTx(ctx, func(tx db.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO room_members (room_id, user_id, role, added_by)
			VALUES ($1, $2, 'owner', 'test')`, room.ID, heir); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE room_members SET revoked_at = NOW()
			WHERE room_id = $1 AND user_id = $2`, room.ID, first)
		return err
	}); err != nil {
		t.Fatalf("transfer: %v", err)
	}

	got, err := repo.GetBySlug(ctx, room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if got.OwnerID == nil || *got.OwnerID != heir {
		t.Fatalf("GetBySlug owner_id = %v, want heir %v", got.OwnerID, heir)
	}
	if listed := ownedSlugs(ctx, t, repo, first); listed[room.Slug] {
		t.Fatal("ListByOwner(former owner) still lists the room")
	}
	if listed := ownedSlugs(ctx, t, repo, heir); !listed[room.Slug] {
		t.Fatal("ListByOwner(heir) does not list the room")
	}

	var heirName string
	if err := pool.QueryRow(ctx, `SELECT display_name FROM users WHERE id = $1`, heir).Scan(&heirName); err != nil {
		t.Fatalf("heir name: %v", err)
	}
	rooms, err := repo.ListFiltered(ctx, db.RoomListParams{Query: room.Slug, Limit: 5})
	if err != nil {
		t.Fatalf("ListFiltered: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("ListFiltered returned %d rooms, want 1", len(rooms))
	}
	if rooms[0].OwnerID == nil || *rooms[0].OwnerID != heir ||
		rooms[0].OwnerDisplayName == nil || *rooms[0].OwnerDisplayName != heirName {
		t.Fatalf("discovery owner = %v / %v, want %v / %q", rooms[0].OwnerID, rooms[0].OwnerDisplayName, heir, heirName)
	}
}

func ownedSlugs(ctx context.Context, t *testing.T, repo *db.RoomRepository, owner uuid.UUID) map[string]bool {
	t.Helper()
	rooms, err := repo.ListByOwner(ctx, owner)
	if err != nil {
		t.Fatalf("ListByOwner: %v", err)
	}
	slugs := map[string]bool{}
	for _, r := range rooms {
		if r.OwnerID == nil {
			t.Errorf("ListByOwner returned room %s without an owner_id", r.Slug)
		}
		slugs[r.Slug] = true
	}
	return slugs
}

// runMigrationFile executes one migration file inside tx (DDL is transactional in
// PostgreSQL, so the caller's rollback restores the schema).
func runMigrationFile(ctx context.Context, t *testing.T, tx db.Tx, name string) {
	t.Helper()
	sql, err := os.ReadFile("../../migrations/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("apply %s: %v", name, err)
	}
}

// The 000097 down migration restores rooms.owner_id from the membership authority, so
// the retirement is reversible without losing any human owner.
func TestRoomsOwnerRetired_DownRestoresOwnerFromMembership(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	owner := insertAuthorityUser(ctx, t, pool, "retdown")
	room := createAuthorityRoom(ctx, t, pool, "rm-ret-down", models.CreateRoomParams{OwnerID: owner})

	tx, err := pool.BeginTx(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background()) //nolint:errcheck
	runMigrationFile(ctx, t, tx, "000097_retire_rooms_owner_id.down.sql")
	var restored *uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT owner_id FROM rooms WHERE id = $1`, room.ID).Scan(&restored); err != nil {
		t.Fatalf("read restored owner_id: %v", err)
	}
	if restored == nil || *restored != owner {
		t.Fatalf("restored owner_id = %v, want %v", restored, owner)
	}
	runMigrationFile(ctx, t, tx, "000097_retire_rooms_owner_id.up.sql")
}

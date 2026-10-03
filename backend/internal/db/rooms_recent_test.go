package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Recent rooms (idx 92 step 1): a person finds the rooms they work in — owned or joined —
// most recently active first, so returning to work starts from where they left off. An
// agent sees its own rooms and its human's OWNED rooms (family access), never a room its
// human merely joined, which the agent could not read.

func insertRecentAgent(ctx context.Context, t *testing.T, pool *db.Pool, tag string, human *uuid.UUID) string {
	t.Helper()
	id := "agent_g1recent_" + tag + "_" + time.Now().Format("150405.000000")[7:]
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, human_id, human_claimed_at)
		VALUES ($1, $1, $2, CASE WHEN $2::uuid IS NULL THEN NULL ELSE NOW() END)`, id, human)
	require.NoError(t, err)
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, id) }) //nolint:errcheck
	return id
}

func setActive(ctx context.Context, t *testing.T, pool *db.Pool, room *models.Room, ago time.Duration) {
	t.Helper()
	_, err := pool.Exec(ctx, `UPDATE rooms SET last_active_at = NOW() - $2::interval WHERE id = $1`,
		room.ID, ago.String())
	require.NoError(t, err)
}

func slugsOf(rooms []models.Room) []string {
	out := make([]string, 0, len(rooms))
	for _, r := range rooms {
		out = append(out, r.Slug)
	}
	return out
}

func TestRoomRepository_ListRecentForCaller_HumanSeesOwnedAndJoinedRoomsByRecentActivity(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	repo := db.NewRoomRepository(pool)
	human := insertAuthorityUser(ctx, t, pool, "recent")
	other := insertAuthorityUser(ctx, t, pool, "recentother")
	tag := time.Now().Format("150405")

	owned := createAuthorityRoom(ctx, t, pool, "rm-rec-owned-"+tag, models.CreateRoomParams{OwnerID: human})
	joined := createAuthorityRoom(ctx, t, pool, "rm-rec-joined-"+tag, models.CreateRoomParams{OwnerID: other, IsPrivate: true})
	_, err := pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id, role, added_by) VALUES ($1, $2, 'member', 'test')`, joined.ID, human)
	require.NoError(t, err)
	left := createAuthorityRoom(ctx, t, pool, "rm-rec-left-"+tag, models.CreateRoomParams{OwnerID: other})
	_, err = pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id, role, added_by, revoked_at) VALUES ($1, $2, 'member', 'test', NOW())`, left.ID, human)
	require.NoError(t, err)
	gone := createAuthorityRoom(ctx, t, pool, "rm-rec-gone-"+tag, models.CreateRoomParams{OwnerID: human})
	require.NoError(t, repo.SoftDelete(ctx, gone.ID))
	unrelated := createAuthorityRoom(ctx, t, pool, "rm-rec-unrel-"+tag, models.CreateRoomParams{OwnerID: other})

	setActive(ctx, t, pool, owned, 48*time.Hour)
	setActive(ctx, t, pool, joined, time.Hour)

	rooms, err := repo.ListRecentForCaller(ctx, db.RecentRoomsCaller{UserID: &human})
	require.NoError(t, err)
	require.Equal(t, []string{joined.Slug, owned.Slug}, slugsOf(rooms),
		"owned and joined rooms, most recently active first; never a left, deleted or unrelated room")
	_ = unrelated
}

func TestRoomRepository_ListRecentForCaller_AgentSeesItsRoomsAndItsHumansOwnedRoomsOnly(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	repo := db.NewRoomRepository(pool)
	human := insertAuthorityUser(ctx, t, pool, "recfam")
	other := insertAuthorityUser(ctx, t, pool, "recfamother")
	agent := insertRecentAgent(ctx, t, pool, "fam", &human)
	tag := time.Now().Format("150405")

	mine := createAuthorityRoom(ctx, t, pool, "rm-rec-agent-"+tag, models.CreateRoomParams{OwnerID: other})
	_, err := pool.Exec(ctx, `INSERT INTO room_members (room_id, agent_id, role, added_by) VALUES ($1, $2, 'member', 'test')`, mine.ID, agent)
	require.NoError(t, err)
	family := createAuthorityRoom(ctx, t, pool, "rm-rec-family-"+tag, models.CreateRoomParams{OwnerID: human, IsPrivate: true})
	humanJoined := createAuthorityRoom(ctx, t, pool, "rm-rec-hjoin-"+tag, models.CreateRoomParams{OwnerID: other, IsPrivate: true})
	_, err = pool.Exec(ctx, `INSERT INTO room_members (room_id, user_id, role, added_by) VALUES ($1, $2, 'member', 'test')`, humanJoined.ID, human)
	require.NoError(t, err)
	setActive(ctx, t, pool, family, 2*time.Hour)
	setActive(ctx, t, pool, mine, time.Hour)

	rooms, err := repo.ListRecentForCaller(ctx, db.RecentRoomsCaller{AgentID: agent, FamilyOwnerID: &human})
	require.NoError(t, err)
	require.Equal(t, []string{mine.Slug, family.Slug}, slugsOf(rooms))

	unclaimed := insertRecentAgent(ctx, t, pool, "solo", nil)
	_, err = pool.Exec(ctx, `INSERT INTO room_members (room_id, agent_id, role, added_by) VALUES ($1, $2, 'member', 'test')`, mine.ID, unclaimed)
	require.NoError(t, err)
	rooms, err = repo.ListRecentForCaller(ctx, db.RecentRoomsCaller{AgentID: unclaimed})
	require.NoError(t, err)
	require.Equal(t, []string{mine.Slug}, slugsOf(rooms), "an unclaimed agent still finds the rooms it works in")
}

func TestRoomRepository_ListRecentForCaller_NobodyListsNothing(t *testing.T) {
	ctx, pool := openAuthorityPool(t)
	rooms, err := db.NewRoomRepository(pool).ListRecentForCaller(ctx, db.RecentRoomsCaller{})
	require.NoError(t, err)
	require.Empty(t, rooms)
}

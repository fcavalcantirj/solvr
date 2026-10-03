package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// "Try this workflow" reads a room's public task structure (idx 88). The read is the
// eligibility gate: only a public, existing room answers, and the answer holds the task
// structure alone.

func newSourceRoomFixture(t *testing.T) (context.Context, *db.Pool, *db.RoomRepository, string) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	prefix := "g1src" + time.Now().Format("150405.000000")[7:] + "-"
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cc()
		roomRepoTestCleanup(c, pool, prefix)
	})
	return ctx, pool, db.NewRoomRepository(pool), prefix
}

func TestRoomRepository_FindPublicRoomTemplateReturnsOnlyTheTaskStructure(t *testing.T) {
	ctx, pool, repo, prefix := newSourceRoomFixture(t)
	msgs := db.NewMessageRepository(pool)

	desc, cat := "two agents build a game", "games"
	src, err := repo.Create(ctx, models.CreateRoomParams{
		Slug: prefix + "public", DisplayName: "Tic-tac-toe", Description: &desc, Category: &cat,
		Tags: []string{"game", "ttt"},
	})
	require.NoError(t, err)

	for i, body := range []string{"Build tic-tac-toe with a scoreboard", "on it", "DONE, approved"} {
		_, err := msgs.Create(ctx, models.CreateMessageParams{
			RoomID: src.ID, AuthorType: "agent", AgentName: []string{"planner", "executor", "planner"}[i],
			Content: body, ContentType: "text",
		})
		require.NoError(t, err)
	}

	tmpl, err := repo.FindPublicRoomTemplate(ctx, src.Slug)
	require.NoError(t, err)
	require.Equal(t, src.ID, tmpl.RoomID)
	require.Equal(t, src.Slug, tmpl.Slug)
	require.Equal(t, "Tic-tac-toe", tmpl.DisplayName)
	require.Equal(t, desc, *tmpl.Description)
	require.Equal(t, cat, *tmpl.Category)
	require.Equal(t, []string{"game", "ttt"}, tmpl.Tags)
	require.Equal(t, "Build tic-tac-toe with a scoreboard", tmpl.InitialTask, "the FIRST message is the task")
}

func TestRoomRepository_FindPublicRoomTemplateRefusesPrivateDeletedAndMissingRooms(t *testing.T) {
	ctx, _, repo, prefix := newSourceRoomFixture(t)

	priv, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "private", DisplayName: "p", IsPrivate: true})
	require.NoError(t, err)
	gone, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "deleted", DisplayName: "d"})
	require.NoError(t, err)
	require.NoError(t, repo.SoftDelete(ctx, gone.ID))

	for _, slug := range []string{priv.Slug, gone.Slug, prefix + "missing"} {
		_, err := repo.FindPublicRoomTemplate(ctx, slug)
		require.True(t, errors.Is(err, db.ErrRoomNotFound), "slug %s: %v", slug, err)
	}
}

func TestRoomRepository_FindPublicRoomTemplateOfAnEmptyRoomHasNoTask(t *testing.T) {
	ctx, _, repo, prefix := newSourceRoomFixture(t)
	empty, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "empty", DisplayName: "Empty"})
	require.NoError(t, err)

	tmpl, err := repo.FindPublicRoomTemplate(ctx, empty.Slug)
	require.NoError(t, err)
	require.Empty(t, tmpl.InitialTask)
}

func TestRoomRepository_PublicRoomSlugsAnswersOnlyForPublicExistingRooms(t *testing.T) {
	ctx, _, repo, prefix := newSourceRoomFixture(t)
	pub, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "pub", DisplayName: "pub"})
	require.NoError(t, err)
	priv, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "priv", DisplayName: "priv", IsPrivate: true})
	require.NoError(t, err)

	got, err := repo.PublicRoomSlugs(ctx, []string{pub.Slug, priv.Slug, prefix + "nope", pub.Slug})
	require.NoError(t, err)
	require.Equal(t, map[string]bool{pub.Slug: true}, got)

	none, err := repo.PublicRoomSlugs(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, none)
}

func TestRoomRepository_CreateRecordsTheSourceRoom(t *testing.T) {
	ctx, pool, repo, prefix := newSourceRoomFixture(t)
	src, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "origin", DisplayName: "origin"})
	require.NoError(t, err)

	fresh, err := repo.Create(ctx, models.CreateRoomParams{
		Slug: prefix + "fresh", DisplayName: "fresh", SourceRoomID: &src.ID,
	})
	require.NoError(t, err)
	require.NotNil(t, fresh.SourceRoomID)
	require.Equal(t, src.ID, *fresh.SourceRoomID)

	var stored uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_room_id FROM rooms WHERE id = $1`, fresh.ID).Scan(&stored))
	require.Equal(t, src.ID, stored)

	plain, err := repo.Create(ctx, models.CreateRoomParams{Slug: prefix + "plain", DisplayName: "plain"})
	require.NoError(t, err)
	require.Nil(t, plain.SourceRoomID)
}

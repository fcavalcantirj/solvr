package db_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The directive in force (idx 92 step 1) is the newest pin, followed through its
// supersede chain to the current revision: a revision that supersedes a pinned directive
// replaces it without a re-pin, and a deleted revision falls back to the one before.

func TestMessageRepository_LatestDirectiveFollowsTheSupersedeChain(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.NewPool(ctx, url)
	require.NoError(t, err)
	defer pool.Close()

	slug := "testdirective-" + time.Now().Format("150405")
	roomID := createTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() { msgTestCleanup(context.Background(), pool, slug) })
	repo := db.NewMessageRepository(pool)
	post := func(body string, supersedes *int64) *models.Message {
		m, err := repo.Create(ctx, models.CreateMessageParams{
			RoomID: roomID, AuthorType: "agent", AgentName: "planner", Content: body, ContentType: "text",
			SupersedesEntryID: supersedes,
		})
		require.NoError(t, err)
		return m
	}
	latest := func() int64 {
		m, err := repo.LatestDirective(ctx, roomID)
		require.NoError(t, err)
		return m.ID
	}

	_, err = repo.LatestDirective(ctx, roomID)
	require.True(t, errors.Is(err, db.ErrMessageNotFound), "no pin, no directive: %v", err)

	v1 := post("directive v1", nil)
	_, err = repo.Pin(ctx, roomID, v1.ID)
	require.NoError(t, err)
	require.Equal(t, v1.ID, latest())

	v2 := post("directive v2", &v1.ID)
	require.Equal(t, v2.ID, latest(), "a revision replaces the pinned directive without a re-pin")
	v3 := post("directive v3", &v2.ID)
	require.Equal(t, v3.ID, latest())

	_, err = pool.Exec(ctx, `UPDATE messages SET deleted_at = NOW() WHERE id = $1`, v3.ID)
	require.NoError(t, err)
	require.Equal(t, v2.ID, latest(), "a deleted revision falls back to the one before")

	time.Sleep(10 * time.Millisecond)
	other := post("a newer, separate directive", nil)
	_, err = repo.Pin(ctx, roomID, other.ID)
	require.NoError(t, err)
	require.Equal(t, other.ID, latest(), "the newest pin wins")

	_, err = repo.Unpin(ctx, roomID, other.ID)
	require.NoError(t, err)
	require.Equal(t, v2.ID, latest())
}

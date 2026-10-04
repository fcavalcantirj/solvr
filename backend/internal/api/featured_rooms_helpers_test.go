package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// featureOnHomepage makes exactly these rooms the homepage's featured pool (SPEC
// Part 26, "Featured rooms"), in the given order. The pool is emptied first so a
// room featured by another test cannot push these out of the three shown. Rooms
// are inserted directly, so a test may feature a room that is (or later turns)
// private to prove it is never shown. The change is announced like a real one,
// so no cached overview hides it.
func featureOnHomepage(t *testing.T, pool *db.Pool, slugs ...string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx, `DELETE FROM featured_rooms`)
	require.NoError(t, err, "empty the featured pool")
	for i, slug := range slugs {
		tag, err := pool.Exec(ctx, `
			INSERT INTO featured_rooms (room_id, featured_at)
			SELECT id, now() + make_interval(secs => $2) FROM rooms WHERE slug = $1`, slug, i)
		require.NoError(t, err, "feature %s", slug)
		require.Equal(t, int64(1), tag.RowsAffected(), "feature %s: the room must exist", slug)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM featured_rooms`) //nolint:errcheck
	})
	require.NoError(t, pool.OverviewChanged(ctx), "announce the featured pool")
}

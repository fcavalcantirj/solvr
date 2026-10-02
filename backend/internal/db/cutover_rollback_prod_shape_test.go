package db

import (
	"context"
	"path/filepath"
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// Task idx 93 step 4 (rollback). Production's schema has no tags or post_tags table: 000024
// was never applied there (measured on the post-purge production copy, 2026-10-02). An up
// migration that only drops their indexes IF EXISTS passes there, so the rollback must also
// run on that shape: a down path that stops halfway leaves production's schema dirty
// mid-chain in the deploy window.
func TestCutoverRollback_DownPathRunsOnProductionShapeWithoutTags(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `DROP TABLE post_tags, tags`)
	require.NoError(t, err, "production's shape: no tags tables")

	// Production after `migrate force 84` and `migrate up`: golang-migrate's table at the head.
	ups, err := filepath.Glob(filepath.Join(backendRoot(t), "migrations", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, ups)
	sort.Strings(ups)
	head, err := strconv.ParseInt(filepath.Base(ups[len(ups)-1])[:6], 10, 64)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO schema_migrations (version, dirty) VALUES ($1, false)`, head)
	require.NoError(t, err)

	require.Positive(t, migrateDownTo84(ctx, t, pool))

	var version int64
	var dirty bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, int64(84), version)
	require.False(t, dirty, "schema_migrations is dirty after the rollback")
	var tags *string
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('tags')::text`).Scan(&tags))
	require.Nil(t, tags, "the rollback must not create the tags table production never had")
}

package db

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// newMigratedScratchDatabase creates an empty database next to DATABASE_URL's, applies every
// up migration to it and returns a pool on it. Platform-wide aggregates (counts, averages,
// top-N lists) are exact there: no row of another test can move them. The database is dropped
// when the test ends. dropLegacy drops the legacy contribution tables (LegacyTables) the way
// schema cleanup will, so a query that still needs them fails.
func newMigratedScratchDatabase(t *testing.T) (pool *Pool, dropLegacy func()) {
	t.Helper()
	base := os.Getenv("DATABASE_URL")
	if base == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	admin, err := pgx.Connect(ctx, base)
	require.NoError(t, err, "connect admin")
	name := fmt.Sprintf("solvr_scratch_%d", time.Now().UnixNano())
	_, err = admin.Exec(ctx, "CREATE DATABASE "+name)
	if err != nil {
		admin.Close(ctx)
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		c, cc := context.WithTimeout(context.Background(), 30*time.Second)
		defer cc()
		if _, err := admin.Exec(c, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
		admin.Close(c)
	})

	u, err := url.Parse(base)
	require.NoError(t, err, "parse DATABASE_URL")
	u.Path = "/" + name
	conn, err := pgx.Connect(ctx, u.String())
	require.NoError(t, err, "connect scratch")
	files, err := filepath.Glob(filepath.Join(backendRoot(t), "migrations", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no up migrations found")
	sort.Strings(files)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		require.NoError(t, err)
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			conn.Close(ctx)
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
	conn.Close(ctx)

	pool, err = NewPool(ctx, u.String())
	require.NoError(t, err, "pool on scratch database")
	t.Cleanup(pool.Close) // registered after the drop, so it runs before it
	return pool, func() {
		t.Helper()
		_, err := pool.Exec(context.Background(), "DROP TABLE "+strings.Join(LegacyTables, ", ")+" CASCADE")
		require.NoError(t, err, "drop the legacy tables")
	}
}

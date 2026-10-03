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
// when the test ends. The legacy tables are already archived there (idx 68): the second
// result only fails the test, pointing at newPreArchiveScratchDatabase, which seeds legacy
// rows and archives them the way production does.
func newMigratedScratchDatabase(t *testing.T) (pool *Pool, archived func()) {
	t.Helper()
	before, after := splitAtLegacyArchive(t)
	pool, _ = newScratchDatabase(t, append(before, after...))
	return pool, func() {
		t.Helper()
		t.Fatal("the legacy tables are archived at head: seed legacy rows on newPreArchiveScratchDatabase")
	}
}

// newPreArchiveScratchDatabase is newMigratedScratchDatabase stopped below the legacy archive
// migration, where the legacy contribution tables (LegacyTables) are still live, so a test can
// seed legacy rows and run the cutover. archiveLegacy then applies the archive migration and
// every later one, the order production follows: seed, cutover, archive. A query that still
// needs a legacy table, a legacy type or a problem-only column fails after it.
func newPreArchiveScratchDatabase(t *testing.T) (pool *Pool, archiveLegacy func()) {
	t.Helper()
	before, after := splitAtLegacyArchive(t)
	pool, url := newScratchDatabase(t, before)
	return pool, func() {
		t.Helper()
		applyMigrationFiles(t, url, after)
		pool.pool.Reset() // no connection keeps a statement prepared against the old schema
	}
}

// splitAtLegacyArchive returns the up migrations below *_legacy_archive.up.sql and the rest,
// in order.
func splitAtLegacyArchive(t *testing.T) (before, after []string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(backendRoot(t), "migrations", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no up migrations found")
	sort.Strings(files)
	for i, f := range files {
		if strings.HasSuffix(f, "_legacy_archive.up.sql") {
			return files[:i], files[i:]
		}
	}
	t.Fatal("no *_legacy_archive.up.sql migration")
	return nil, nil
}

// newScratchDatabase creates the database, applies files and returns a pool and its URL.
func newScratchDatabase(t *testing.T, files []string) (*Pool, string) {
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
	applyMigrationFiles(t, u.String(), files)

	pool, err := NewPool(ctx, u.String())
	require.NoError(t, err, "pool on scratch database")
	t.Cleanup(pool.Close) // registered after the drop, so it runs before it
	return pool, u.String()
}

// applyMigrationFiles runs each file, in order, as one multi-statement exec (one transaction).
func applyMigrationFiles(t *testing.T, dbURL string, files []string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dbURL)
	require.NoError(t, err, "connect scratch")
	defer conn.Close(ctx)
	for _, f := range files {
		sql, err := os.ReadFile(f)
		require.NoError(t, err)
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("apply %s: %v", filepath.Base(f), err)
		}
	}
}

package jobs_test

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Every static SQL statement of the non-test sources is prepared on the scratch database
// below the legacy archive migration and again after the cutover and the archive. A
// statement that prepares before and fails after reaches dropped legacy storage (a legacy
// table, its row type, or a posts column the archive moved to legacy_archive.post_fields):
// its file must be found by the regex source scan and carry a pending (non-keep, not done)
// code: disposition, or be one of preArchiveCutoverFiles. This checks the scan's blind spots
// (aliases, line breaks, constants) with the database's own name resolution instead of
// another regex.
// preArchiveCutoverFiles are the cutover tool's files (cmd/cutover): owner decision 2026-10-02
// (idx 68) keeps them as the operator and rehearsal tool for databases below the legacy
// archive, so they read the legacy tables by design and carry keep dispositions.
// post_migration.go (the cutover's post-state remap and content fingerprint) reads only the
// archived posts columns, which the table-name scan does not look for, so it has no
// disposition of its own.
var preArchiveCutoverFiles = map[string]bool{
	"code:internal/db/contribution_migration.go":   true,
	"code:internal/db/knowledge_cutover.go":        true,
	"code:internal/db/knowledge_cutover_search.go": true,
	"code:internal/db/legacy_relation_remap.go":    true,
	"code:internal/db/post_migration.go":           true,
	"code:internal/db/reputation_history.go":       true,
}

func TestLegacyDroppedDatabase_StaticQueriesExposeOnlyRegisteredDependencies(t *testing.T) {
	stmts, err := db.ExtractStaticSQL("../..")
	if err != nil {
		t.Fatalf("extract static sql: %v", err)
	}
	before := make([]error, len(stmts))
	d := newLegacyDroppedDatabase(t, func(ctx context.Context, conn *pgx.Conn) {
		for i, s := range stmts {
			_, before[i] = conn.PgConn().Prepare(ctx, "", s.SQL, nil)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, d.url)
	if err != nil {
		t.Fatalf("connect scratch after drop: %v", err)
	}
	defer conn.Close(ctx)

	src, err := db.ScanLegacySourceDependencies("../..")
	if err != nil {
		t.Fatalf("scan sources: %v", err)
	}
	scanned := map[string]bool{}
	for _, dep := range src {
		scanned[dep.Key] = true
	}

	prepared := 0
	unpreparable := map[string]int{}
	failing := map[string][]string{} // code:<file> -> "<line>:<dropped relation>"
	for i, s := range stmts {
		if before[i] != nil {
			unpreparable[s.File]++
			continue
		}
		prepared++
		_, err := conn.PgConn().Prepare(ctx, "", s.SQL, nil)
		if err == nil {
			continue
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Fatalf("%s:%d: prepare after the drop: %v", s.File, s.Line, err)
		}
		rel, ok := missingLegacyObject(tracedError{Code: pgErr.Code, Message: pgErr.Message})
		if !ok {
			t.Errorf("%s:%d prepared before the drop but fails after it for an unrelated reason: %s %s",
				s.File, s.Line, pgErr.Code, pgErr.Message)
			continue
		}
		key := "code:" + s.File
		failing[key] = append(failing[key], fmt.Sprintf("%d:%s", s.Line, rel))
	}
	t.Logf("static statements: %d extracted, %d prepared on the migrated schema, %d did not prepare there",
		len(stmts), prepared, len(stmts)-prepared)
	t.Logf("did not prepare before the drop (runtime-built or other-database SQL), per file: %v", unpreparable)

	if prepared == 0 {
		t.Fatal("no static statement prepared on the migrated schema: the probe checks nothing")
	}
	if len(failing["code:internal/db/contribution_migration.go"]) == 0 {
		t.Error("positive control: the cutover's contribution migration must fail once approaches is dropped")
	}

	keys := make([]string, 0, len(failing))
	for key := range failing {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t.Logf("%s: %d statements reach dropped relations: %v", key, len(failing[key]), failing[key])
		if key == "code:internal/db/post_migration.go" && onlyArchivedPostColumns(failing[key]) {
			continue // the cutover's post fingerprint: archived posts columns only, by design
		}
		if !scanned[key] {
			t.Errorf("%s reaches dropped legacy relations %v but the regex source scan does not find it: a hidden legacy dependency",
				key, failing[key])
		}
		disp, ok := db.LegacyDependencyDispositions[key]
		switch {
		case !ok:
			t.Errorf("%s reaches dropped legacy relations but has no disposition", key)
		case disp.Action == db.LegacyActionKeep && preArchiveCutoverFiles[key]:
		case disp.Action == db.LegacyActionKeep || disp.Done:
			t.Errorf("%s reaches dropped legacy relations %v but is recorded as %s (done=%v)", key, failing[key], disp.Action, disp.Done)
		}
	}

	// Informational: pending code entries whose legacy SQL is built at runtime, so no static
	// statement of theirs failed here; the regex scan remains their only evidence.
	var unseen []string
	for key, disp := range db.LegacyDependencyDispositions {
		if strings.HasPrefix(key, "code:") && disp.Action != db.LegacyActionKeep && !disp.Done && len(failing[key]) == 0 {
			unseen = append(unseen, key)
		}
	}
	sort.Strings(unseen)
	t.Logf("pending code entries with no failing static statement: %v", unseen)
}

// onlyArchivedPostColumns reports whether every "<line>:<relation>" entry names a posts column
// the legacy archive migration moved out of posts.
func onlyArchivedPostColumns(entries []string) bool {
	for _, e := range entries {
		_, rel, _ := strings.Cut(e, ":")
		if !archivedPostColumns[rel] {
			return false
		}
	}
	return len(entries) > 0
}

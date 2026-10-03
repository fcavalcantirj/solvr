package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// lastMigrationBeforeLegacyArchive is the highest NNNNNN prefix among backend/migrations/
// *.up.sql below *_legacy_archive.up.sql: the last schema where the legacy tables are live,
// and so the only version the cutover may run at. Migrations added after the archive do not
// move it.
func lastMigrationBeforeLegacyArchive(t *testing.T) int64 {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no up migrations found in backend/migrations (%v)", err)
	}
	version := func(f string) int64 {
		prefix, _, _ := strings.Cut(filepath.Base(f), "_")
		n, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			t.Fatalf("migration %s: prefix %q is not a version number", f, prefix)
		}
		return n
	}
	var archive int64
	for _, f := range files {
		if strings.HasSuffix(f, "_legacy_archive.up.sql") {
			archive = version(f)
		}
	}
	if archive == 0 {
		t.Fatal("no *_legacy_archive.up.sql migration")
	}
	var last int64
	for _, f := range files {
		if n := version(f); n < archive && n > last {
			last = n
		}
	}
	return last
}

func TestParseOptions_ExpectsTheLastMigrationBeforeTheLegacyArchive(t *testing.T) {
	opts, err := parseOptions([]string{"--database-url", "postgres://x/db", "--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := lastMigrationBeforeLegacyArchive(t); opts.expectVersion != want {
		t.Fatalf("--expect-version defaults to %d, the last migration before the legacy archive is %d: the cutover runs only there", opts.expectVersion, want)
	}
}

func TestParseOptions_GuardsTheProductionRun(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"database url is explicit, never taken from the environment", []string{"--dry-run"}, "--database-url"},
		{"applying needs the confirmation flag", []string{"--database-url", "postgres://x/db"}, "--confirm-prod"},
		{"a dry run needs no confirmation", []string{"--database-url", "postgres://x/db", "--dry-run"}, ""},
		{"a confirmed apply is accepted", []string{"--database-url", "postgres://x/db", "--confirm-prod"}, ""},
		{"the expected schema version must be positive", []string{"--database-url", "postgres://x/db", "--dry-run", "--expect-version", "0"}, "--expect-version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://from-env/should-not-be-used")
			opts, err := parseOptions(tc.args)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if opts.databaseURL != "postgres://x/db" {
					t.Fatalf("database url %q, want the flag's value", opts.databaseURL)
				}
				if want := lastMigrationBeforeLegacyArchive(t); opts.expectVersion != want {
					t.Fatalf("expect version %d, want the default, the last migration before the legacy archive %d", opts.expectVersion, want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %v, want one naming %s", err, tc.wantErr)
			}
		})
	}
}

func TestCheckSchema_RefusesAnythingButTheExpectedCleanVersion(t *testing.T) {
	if err := checkSchema(132, false, 132); err != nil {
		t.Fatalf("clean 132: %v", err)
	}
	if err := checkSchema(132, true, 132); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty 132 accepted: %v", err)
	}
	if err := checkSchema(84, false, 132); err == nil || !strings.Contains(err.Error(), "84") {
		t.Fatalf("version 84 accepted: %v", err)
	}
}

func TestParseOptions_SamplesTheMostFrequentSearchesByDefault(t *testing.T) {
	base := []string{"--database-url", "postgres://x/db", "--dry-run"}
	opts, err := parseOptions(base)
	if err != nil || opts.searchSample != 200 {
		t.Fatalf("search sample %d (%v), want the default 200", opts.searchSample, err)
	}
	opts, err = parseOptions(append(base, "--search-sample", "0"))
	if err != nil || opts.searchSample != 0 {
		t.Fatalf("search sample %d (%v), want 0 (no comparison)", opts.searchSample, err)
	}
	if _, err := parseOptions(append(base, "--search-sample", "-1")); err == nil || !strings.Contains(err.Error(), "--search-sample") {
		t.Fatalf("error %v, want one naming --search-sample", err)
	}
}

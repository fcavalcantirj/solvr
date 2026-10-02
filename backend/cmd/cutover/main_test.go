package main

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// highestMigration is the highest NNNNNN prefix among backend/migrations/*.up.sql: the version
// `migrate up` leaves a database at, and so the version the cutover must expect by default.
func highestMigration(t *testing.T) int64 {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "migrations", "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no up migrations found in backend/migrations (%v)", err)
	}
	var highest int64
	for _, f := range files {
		prefix, _, _ := strings.Cut(filepath.Base(f), "_")
		n, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			t.Fatalf("migration %s: prefix %q is not a version number", f, prefix)
		}
		if n > highest {
			highest = n
		}
	}
	return highest
}

func TestParseOptions_ExpectsTheHighestMigrationByDefault(t *testing.T) {
	opts, err := parseOptions([]string{"--database-url", "postgres://x/db", "--dry-run"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := highestMigration(t); opts.expectVersion != want {
		t.Fatalf("--expect-version defaults to %d, the highest migration is %d: bump the default with every migration", opts.expectVersion, want)
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
				if want := highestMigration(t); opts.expectVersion != want {
					t.Fatalf("expect version %d, want the default, the highest migration %d", opts.expectVersion, want)
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

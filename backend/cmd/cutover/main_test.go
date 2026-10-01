package main

import (
	"strings"
	"testing"
)

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
				if opts.expectVersion != 130 {
					t.Fatalf("expect version %d, want the default 130", opts.expectVersion)
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
	if err := checkSchema(130, false, 130); err != nil {
		t.Fatalf("clean 130: %v", err)
	}
	if err := checkSchema(130, true, 130); err == nil || !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("dirty 130 accepted: %v", err)
	}
	if err := checkSchema(84, false, 130); err == nil || !strings.Contains(err.Error(), "84") {
		t.Fatalf("version 84 accepted: %v", err)
	}
}

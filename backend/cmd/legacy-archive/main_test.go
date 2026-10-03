package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOptions_TheDatabaseIsAFlagNeverTheEnvironment(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://from-env/should-not-be-used")
	if _, err := parseOptions(nil); err == nil || !strings.Contains(err.Error(), "--database-url") {
		t.Fatalf("error %v, want one naming --database-url", err)
	}
	opts, err := parseOptions([]string{"--database-url", "postgres://x/db"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.databaseURL != "postgres://x/db" || opts.exportPath != "" {
		t.Fatalf("options %+v, want the flag's url and no export", opts)
	}
	opts, err = parseOptions([]string{"--database-url", "postgres://x/db", "--export", "/tmp/archive.jsonl"})
	if err != nil || opts.exportPath != "/tmp/archive.jsonl" {
		t.Fatalf("options %+v (%v), want the export path", opts, err)
	}
}

// The checksum file is sha256sum's format, so `sha256sum -c` verifies the export where it lies.
func TestWriteChecksumFile_IsSha256sumFormat(t *testing.T) {
	dir := t.TempDir()
	export := filepath.Join(dir, "legacy-archive.jsonl")
	if err := writeChecksumFile(export, "abc123"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(export + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	if want := "abc123  legacy-archive.jsonl\n"; string(got) != want {
		t.Fatalf("checksum file %q, want %q", got, want)
	}
}

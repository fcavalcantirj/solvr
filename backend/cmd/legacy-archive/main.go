// Command legacy-archive reads the recovery archive the legacy archive migration (000138,
// task idx 68) writes and never writes to the database: every read runs in one read-only
// repeatable-read transaction.
//
//	legacy-archive --database-url <url>                      # manifest vs recomputed digests
//	legacy-archive --database-url <url> --export <file>      # plus a checksummed JSON Lines export
//
// It prints one line per archived table: the manifest's row count and sha256 beside the ones
// recomputed from the archived rows, and OK or MISMATCH. The export (written only when every
// table matches) is the archive as JSON Lines, a header with the manifest and then one line
// per row, beside <file>.sha256 in sha256sum format. Exit status: 0 all tables match, 1 a
// mismatch or a failed read, 2 a usage error or a database without the archive.
//
// The database URL is a flag on purpose: DATABASE_URL is never read.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

type options struct {
	databaseURL string
	exportPath  string
}

func parseOptions(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("legacy-archive", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.databaseURL, "database-url", "", "database to read (required; DATABASE_URL is ignored)")
	fs.StringVar(&o.exportPath, "export", "", "write the archive as JSON Lines to this file, and <file>.sha256")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if o.databaseURL == "" {
		return o, errors.New("--database-url is required (DATABASE_URL is deliberately ignored)")
	}
	return o, nil
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "legacy-archive:", err)
		os.Exit(2)
	}
	os.Exit(run(opts, os.Stdout))
}

func run(opts options, out io.Writer) int {
	ctx := context.Background()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.NewPool(connectCtx, opts.databaseURL)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "legacy-archive: connect:", err)
		return 1
	}
	defer pool.Close()

	entries, err := db.ReadLegacyArchive(ctx, pool)
	if errors.Is(err, db.ErrNoLegacyArchive) {
		fmt.Fprintln(os.Stderr, "legacy-archive:", err)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "legacy-archive:", err)
		return 1
	}
	ok := true
	for _, e := range entries {
		verdict := "OK"
		if !e.OK() {
			verdict, ok = "MISMATCH", false
		}
		fmt.Fprintf(out, "%-24s manifest %8d %s  recomputed %8d %s  %s\n",
			e.Table, e.ManifestRows, e.ManifestSHA256, e.Rows, e.SHA256, verdict)
	}
	if !ok {
		fmt.Fprintln(os.Stderr, "legacy-archive:", db.ErrLegacyArchiveMismatch)
		return 1
	}
	if opts.exportPath == "" {
		return 0
	}
	if err := export(ctx, pool, opts.exportPath, out); err != nil {
		fmt.Fprintln(os.Stderr, "legacy-archive: export:", err)
		return 1
	}
	return 0
}

// export writes the archive to path (created, never overwritten) and its checksum file.
func export(ctx context.Context, pool *db.Pool, path string, out io.Writer) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	exp, err := db.ExportLegacyArchive(ctx, pool, f)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := writeChecksumFile(path, exp.SHA256); err != nil {
		return err
	}
	fmt.Fprintf(out, "exported %d rows to %s (sha256 %s, checksum in %s.sha256)\n", exp.Rows, path, exp.SHA256, path)
	return nil
}

// writeChecksumFile writes <path>.sha256 in sha256sum format ("<hex>  <basename>").
func writeChecksumFile(path, sum string) error {
	return os.WriteFile(path+".sha256", []byte(sum+"  "+filepath.Base(path)+"\n"), 0o600)
}

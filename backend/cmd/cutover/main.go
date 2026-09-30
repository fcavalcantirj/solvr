// Command cutover runs the knowledge-model cutover (task idx 93) against one database:
// post states, legacy contributions to replies, the relations that name them, and the
// vote-score and room-activity rebuilds, recorded in cutover_ledger. Run it only after
// `migrate up` has brought the schema to --expect-version and with the API stopped.
//
//	cutover --database-url <url> --dry-run            # read-only: what would change
//	cutover --database-url <url> --confirm-prod       # apply
//
// The database URL is a flag on purpose: DATABASE_URL is never read, so a shell that points
// at production by default cannot be converted by accident.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

type options struct {
	databaseURL   string
	dryRun        bool
	confirmProd   bool
	expectVersion int64
	reportPath    string
}

func parseOptions(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("cutover", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.databaseURL, "database-url", "", "database to convert (required; DATABASE_URL is ignored)")
	fs.BoolVar(&o.dryRun, "dry-run", false, "only read: report what the cutover would change")
	fs.BoolVar(&o.confirmProd, "confirm-prod", false, "required to apply (not needed with --dry-run)")
	fs.Int64Var(&o.expectVersion, "expect-version", 114, "schema_migrations version the database must be at, clean")
	fs.StringVar(&o.reportPath, "report", "", "write the JSON report to this file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	switch {
	case o.databaseURL == "":
		return o, errors.New("--database-url is required (DATABASE_URL is deliberately ignored)")
	case !o.dryRun && !o.confirmProd:
		return o, errors.New("applying the cutover requires --confirm-prod (or use --dry-run)")
	case o.expectVersion <= 0:
		return o, errors.New("--expect-version must be a positive migration version")
	}
	return o, nil
}

func checkSchema(version int64, dirty bool, want int64) error {
	if dirty {
		return fmt.Errorf("schema_migrations is dirty at %d: fix the failed migration first", version)
	}
	if version != want {
		return fmt.Errorf("schema is at %d, the cutover needs %d: run `migrate up` first", version, want)
	}
	return nil
}

func main() {
	opts, err := parseOptions(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "cutover:", err)
		os.Exit(2)
	}
	os.Exit(run(opts))
}

func run(opts options) int {
	ctx := context.Background()
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.NewPool(connectCtx, opts.databaseURL)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cutover: connect:", err)
		return 1
	}
	defer pool.Close()

	version, dirty, err := db.SchemaVersion(ctx, pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cutover: read schema_migrations:", err)
		return 2
	}
	if err := checkSchema(version, dirty, opts.expectVersion); err != nil {
		fmt.Fprintln(os.Stderr, "cutover:", err)
		return 2
	}

	rep, runErr := db.RunKnowledgeCutover(ctx, pool, db.KnowledgeCutoverOptions{DryRun: opts.dryRun})
	if err := writeReport(opts.reportPath, rep); err != nil {
		fmt.Fprintln(os.Stderr, "cutover: report:", err)
		return 1
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, "cutover:", runErr)
		return 1
	}
	return 0
}

func writeReport(path string, rep *db.KnowledgeCutoverReport) error {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if path == "" {
		_, err = os.Stdout.Write(b)
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// Command seo-report writes the weekly SEO report (task idx 85, SPEC.md Part 27.6): the
// server baseline from GET /admin/seo/baseline joined with Google Search Console
// Performance exports. It reads files only; it never calls an API or a database.
//
//	go run ./cmd/seo-report -baseline baseline.json [-pages Pages.csv] [-queries Queries.csv] \
//	    [-brand solvr] [-release 2026-10-02]
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	var o options
	flag.StringVar(&o.baseline, "baseline", "", "JSON saved from GET /admin/seo/baseline (required)")
	flag.StringVar(&o.pages, "pages", "", "Search Console Performance export, Pages (CSV)")
	flag.StringVar(&o.queries, "queries", "", "Search Console Performance export, Queries (CSV)")
	flag.StringVar(&o.brands, "brand", "solvr", "comma-separated brand terms")
	flag.StringVar(&o.release, "release", "2026-10-02", "release date the reviews count from (YYYY-MM-DD)")
	flag.Parse()
	if o.baseline == "" {
		fmt.Fprintln(os.Stderr, "seo-report: -baseline is required")
		os.Exit(2)
	}
	if err := run(os.Stdout, o); err != nil {
		fmt.Fprintln(os.Stderr, "seo-report:", err)
		os.Exit(1)
	}
}

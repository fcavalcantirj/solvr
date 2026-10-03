package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 85: the weekly SEO report joins Search Console exports with the server
// baseline. Segments are kept apart (brand queries, guides, docs, rooms, posts), the
// milestones stay distinct, and what no input covers is UNAVAILABLE, never zero.

func TestParseGSC_ReadsSearchConsoleExports(t *testing.T) {
	f, err := os.Open("testdata/queries.csv")
	require.NoError(t, err)
	defer f.Close()
	rows, err := parseGSC(f)
	require.NoError(t, err)
	require.Len(t, rows, 4)
	assert.Equal(t, gscRow{Key: "connect two ai agents", Clicks: 10, Impressions: 600, Position: 9}, rows[2])
}

func TestSegments_KeepBrandGuidesRoomsAndPostsApart(t *testing.T) {
	assert.Equal(t, "brand", querySegment("Solvr Dev", []string{"solvr"}))
	assert.Equal(t, "non-brand", querySegment("connect two ai agents", []string{"solvr"}))
	for url, want := range map[string]string{
		"https://solvr.dev/": "home",
		"https://solvr.dev/docs/guides/connect-planner-executor": "guides",
		"https://solvr.dev/docs":                                 "docs",
		"https://solvr.dev/rooms/kestrel-room":                   "rooms",
		"https://solvr.dev/rooms/kestrel-room/history/2":         "room transcripts",
		"https://solvr.dev/posts/p1/replies/2":                   "posts",
		"https://solvr.dev/about":                                "other",
	} {
		assert.Equal(t, want, pageSegment(url), url)
	}
}

func TestReviewDates_AreOneFourAndEightWeeksAfterRelease(t *testing.T) {
	release := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, []string{"2026-10-09", "2026-10-30", "2026-11-27"}, reviewDates(release))
}

func TestReport_WithEveryInput(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, run(&out, options{
		baseline: "testdata/baseline.json", pages: "testdata/pages.csv", queries: "testdata/queries.csv",
		brands: "solvr", release: "2026-10-02",
	}))
	report := out.String()
	for _, want := range []string{
		"publication            measured     628",
		"sitemap acceptance     UNAVAILABLE  Google Search Console: Sitemaps",
		"indexing               PARTIAL      7 pages with impressions in the export",
		"traffic                measured     67 clicks, 2070 impressions",
		"activation             measured     4 rooms activated",
		"brand                  38 clicks  600 impressions  CTR 6.33%",
		"non-brand              14 clicks  900 impressions  CTR 1.56%",
		"guides                 12 clicks  300 impressions",
		"/rooms/kestrel-room    6 clicks  3 connections  2 activations",
		"2026-10-09, 2026-10-30, 2026-11-27",
	} {
		assert.Contains(t, report, want)
	}
}

func TestReport_WithoutSearchConsoleSaysUnavailable(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, run(&out, options{baseline: "testdata/baseline.json", brands: "solvr", release: "2026-10-02"}))
	report := out.String()
	assert.Contains(t, report, "traffic                UNAVAILABLE")
	assert.Contains(t, report, "indexing               UNAVAILABLE")
	assert.NotContains(t, report, "0 clicks")
	assert.True(t, strings.Contains(report, "no Search Console export given"))
}

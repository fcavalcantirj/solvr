package db

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// One counting rule, in one place (SPEC.md 27.6): a report reads funnel steps and searches
// through counted_funnel_events and counted_search_queries, which leave out the rows classed
// crawler or automation and the searches a link brought. Rows are flagged, never deleted.

// rawTrafficReaders are the only code that may read funnel_events or search_queries directly,
// keyed "file:Receiver.Func" (or "file:*" for a whole file), with the reason.
var rawTrafficReaders = map[string]string{
	"internal/db/funnel_events.go:FunnelEventRepository.FlowKnown":                 "ingest: a room keeps a flow code only if a step already carries it",
	"internal/db/funnel_events.go:FunnelEventRepository.RecordParticipantJoined":   "the write path: the joiner's ordinal and the room's flow",
	"internal/db/funnel_events.go:FunnelEventRepository.RecordFirstTwoWayExchange": "the write path: the room's flow and source",
	"internal/db/funnel_events.go:FunnelEventRepository.list":                      "one flow's or one room's own steps, for tests and internal analysis",
	"internal/db/search_analytics.go:SearchAnalyticsRepository.DeleteOlderThan":    "retention",
	"internal/db/traffic_rules.go:*":                                               "the behavioural rules judge every row",
	"internal/db/traffic_report.go:*":                                              "the report of what the views leave out, and why",
	"internal/db/seo_baseline.go:SEOBaselineRepository.measureExcluded":            "the baseline's account of what the views left out",
}

var rawTrafficReadRe = regexp.MustCompile(`(?i)\b(FROM|JOIN)\s+(funnel_events|search_queries)\b`)

// TestCountedViews_NoReportReadsTheRawTables reads every string literal of the production Go
// code under internal/ and cmd/. A new reader of the raw tables fails here until it reads the
// counted_* views, or is listed above with its reason.
func TestCountedViews_NoReportReadsTheRawTables(t *testing.T) {
	root := backendRoot(t)
	fset := token.NewFileSet()
	found := map[string]bool{}
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			mark := func(symbol string, node ast.Node) {
				ast.Inspect(node, func(n ast.Node) bool {
					if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && rawTrafficReadRe.MatchString(lit.Value) {
						if _, whole := rawTrafficReaders[rel+":*"]; whole {
							found[rel+":*"] = true
						} else {
							found[rel+":"+symbol] = true
						}
					}
					return true
				})
			}
			for _, decl := range file.Decls {
				switch decl := decl.(type) {
				case *ast.FuncDecl:
					name := decl.Name.Name
					if decl.Recv != nil && len(decl.Recv.List) == 1 {
						if star, ok := decl.Recv.List[0].Type.(*ast.StarExpr); ok {
							if id, ok := star.X.(*ast.Ident); ok {
								name = id.Name + "." + name
							}
						}
					}
					mark(name, decl)
				case *ast.GenDecl:
					for _, spec := range decl.Specs {
						if vs, ok := spec.(*ast.ValueSpec); ok {
							for _, n := range vs.Names {
								mark(n.Name, vs)
							}
						}
					}
				}
			}
			return nil
		})
		require.NoError(t, err)
	}

	var unlisted, unused []string
	for key := range found {
		if _, ok := rawTrafficReaders[key]; !ok {
			unlisted = append(unlisted, key)
		}
	}
	for key := range rawTrafficReaders {
		if !found[key] {
			unused = append(unused, key)
		}
	}
	sort.Strings(unlisted)
	sort.Strings(unused)
	assert.Empty(t, unlisted, "these read funnel_events or search_queries: read counted_funnel_events / counted_search_queries")
	assert.Empty(t, unused, "listed as raw readers but no longer read the raw tables: drop them from the list")
}

type countedFixture struct {
	t    *testing.T
	ctx  context.Context
	pool *Pool
}

func newCountedFixture(t *testing.T) *countedFixture {
	t.Helper()
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return &countedFixture{t: t, ctx: ctx, pool: pool}
}

func (f *countedFixture) exec(sql string, args ...any) {
	f.t.Helper()
	_, err := f.pool.Exec(f.ctx, sql, args...)
	require.NoError(f.t, err, sql)
}

func (f *countedFixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	require.NoError(f.t, f.pool.QueryRow(f.ctx, sql, args...).Scan(&n), sql)
	return n
}

// search stores one public search of query with a class and an entry ("" leaves it NULL).
func (f *countedFixture) search(query, class, entry string, at time.Time) {
	f.t.Helper()
	f.exec(`INSERT INTO search_queries (query, query_normalized, results_count, search_method, duration_ms, searcher_type,
	                                    searched_at, public_scope, traffic_class, entry)
	        VALUES ($1, $1, 2, 'hybrid', 120, 'anonymous', $2, true, $3, NULLIF($4, ''))`, query, at, class, entry)
}

func TestCountedViews_LeaveOutCrawlersAutomationAndLinkedSearches(t *testing.T) {
	f := newCountedFixture(t)
	at := time.Now().Add(-time.Hour)
	for _, class := range []string{"unclassified", "human", "agent", "crawler", "automation"} {
		f.exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, traffic_class, occurred_at)
		        VALUES ('counted_' || $1, 'connection_started', 'browser', 'anonymous', $1, $2)`, class, at)
	}
	for _, s := range []struct{ class, entry string }{
		{"unclassified", ""}, {"human", "typed"}, {"human", "link"}, {"agent", "api"}, {"crawler", "typed"}, {"automation", ""},
	} {
		f.search("q "+s.class+" "+s.entry, s.class, s.entry, at)
	}

	assert.Equal(t, 3, f.count(`SELECT count(*) FROM counted_funnel_events`), "unclassified, human and agent count")
	assert.Equal(t, 0, f.count(`SELECT count(*) FROM counted_funnel_events WHERE traffic_class IN ('crawler', 'automation')`))
	assert.Equal(t, 3, f.count(`SELECT count(*) FROM counted_search_queries`), "unclassified, typed by a person, and the API's")
	assert.Equal(t, 0, f.count(`SELECT count(*) FROM counted_search_queries WHERE entry = 'link'`))
	assert.Equal(t, 5, f.count(`SELECT count(*) FROM funnel_events`), "flagged, never deleted")
	assert.Equal(t, 6, f.count(`SELECT count(*) FROM search_queries`))
}

// Answer 4 (REVIEW-r1): the public figures read the counted views too. The homepage's search
// totals, GET /v1/stats/search and GET /v1/data/* count the same searches the operator does.
func TestCountedViews_ThePublicFiguresCountOnlyCountedSearches(t *testing.T) {
	f := newCountedFixture(t)
	at := time.Now().Add(-time.Hour)
	f.search("kubernetes ingress", "human", "typed", at)
	f.search("deadlock", "agent", "api", at)
	f.search("scraping", "human", "link", at)                  // a tag link brought it
	f.search("openclaw agent heartbeat", "automation", "", at) // a repeated query
	f.search("edge-tts", "crawler", "", at)

	summary, err := NewSearchAnalyticsRepository(f.pool).GetSummary(f.ctx, 7)
	require.NoError(t, err)
	assert.Equal(t, 2, summary.TotalSearches, "GET /v1/stats/search")

	breakdown, err := NewDataAnalyticsRepository(f.pool).GetBreakdown(f.ctx, "24h", false)
	require.NoError(t, err)
	assert.Equal(t, 2, breakdown.TotalSearches, "GET /v1/data/breakdown")

	trending, err := NewDataAnalyticsRepository(f.pool).GetTrendingPublic(f.ctx, "24h", 10, false)
	require.NoError(t, err)
	var terms []string
	for _, tr := range trending {
		terms = append(terms, tr.Query)
	}
	assert.ElementsMatch(t, []string{"kubernetes ingress", "deadlock"}, terms, "GET /v1/data/trending")

	week, ok := RoomStatsWindowByValue("7d")
	require.True(t, ok)
	pulse, err := NewHomepageRepository(f.pool).GetSearchPulse(f.ctx, week, 10)
	require.NoError(t, err)
	assert.Equal(t, 2, pulse.Eligible, "the homepage's search total")
}

// /admin/seo/baseline counts only counted funnel steps and searches, and says what it left out.
func TestCountedViews_TheSEOBaselineCountsCountedRowsAndNamesTheRest(t *testing.T) {
	f := newCountedFixture(t)
	at := time.Now().Add(-time.Hour)
	for i, class := range []string{"human", "human", "automation", "automation", "automation", "crawler"} {
		reason := map[string]string{"human": "browser", "automation": "fetched_without_copy", "crawler": "declared_bot"}[class]
		f.exec(`INSERT INTO funnel_events (flow_id, event_name, source_channel, actor_type, entry_surface, traffic_class, traffic_reason, occurred_at)
		        VALUES ($1, 'connection_started', 'browser', 'anonymous', 'connect_page', $2, $3, $4)`,
			"seo_"+string(rune('a'+i)), class, reason, at)
	}
	f.search("typed words", "human", "typed", at)
	f.search("a tag", "human", "link", at)

	b, err := NewSEOBaselineRepository(f.pool).Measure(f.ctx, at.Add(-time.Hour), at.Add(time.Hour), 50)
	require.NoError(t, err)
	assert.Contains(t, b.Funnel, SEOFunnelCount{Event: models.FunnelConnectionStarted, EntrySurface: "connect_page", Count: 2})
	assert.Equal(t, 1, b.Search.Queries)
	assert.ElementsMatch(t, []SEOExcludedCount{
		{Table: "funnel_events", Event: models.FunnelConnectionStarted, Class: "automation", Reason: "fetched_without_copy", Count: 3},
		{Table: "funnel_events", Event: models.FunnelConnectionStarted, Class: "crawler", Reason: "declared_bot", Count: 1},
		{Table: "search_queries", Class: "human", Reason: "", Entry: "link", Count: 1},
	}, b.Traffic.Excluded)
}

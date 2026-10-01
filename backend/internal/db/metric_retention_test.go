package db

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RawEventSources (metric_retention.go) states how long each raw event table keeps its rows and
// the longest window every metric reads from it. These tests hold that statement to the readers'
// windows, to the code that deletes rows, and to a clean installation's schema, and pin the rule
// that a distinct count over a window is exact rather than a sum of per-bucket distinct counts.

func TestRawEventSources_RetentionCoversEveryReader(t *testing.T) {
	require.NotEmpty(t, RawEventSources)
	seen := map[string]bool{}
	for _, s := range RawEventSources {
		require.False(t, seen[s.Table], "%s is listed twice", s.Table)
		seen[s.Table] = true
		require.NotEmpty(t, s.TimeColumn, "%s names no time column", s.Table)
		require.NotEmpty(t, s.Readers, "%s: a table no metric reads is not a metric source", s.Table)
		readers := map[string]bool{}
		for _, m := range s.Readers {
			require.False(t, readers[m.Reader], "%s: reader %s is listed twice", s.Table, m.Reader)
			readers[m.Reader] = true
			require.NotEmpty(t, m.Metric, "%s: %s states no metric", s.Table, m.Reader)
			if s.Retention == KeptIndefinitely {
				continue
			}
			assert.NotEqual(t, AllHistory, m.Window,
				"%s keeps rows for %s, but %s reads all of its history", s.Table, s.Retention, m.Reader)
			assert.LessOrEqual(t, m.Window, s.Retention,
				"%s keeps rows for %s, but %s reads %s", s.Table, s.Retention, m.Reader, m.Window)
		}
	}
}

// readerWindow returns the window RawEventSources states for one reader of one table.
func readerWindow(t *testing.T, table, reader string) time.Duration {
	t.Helper()
	for _, s := range RawEventSources {
		if s.Table != table {
			continue
		}
		for _, m := range s.Readers {
			if m.Reader == reader {
				return m.Window
			}
		}
	}
	t.Fatalf("RawEventSources lists no reader %s of %s", reader, table)
	return 0
}

func TestRawEventSources_StateTheWindowsTheReadersOffer(t *testing.T) {
	var longest time.Duration
	for _, w := range RoomStatsWindows {
		if w.Duration > longest {
			longest = w.Duration
		}
	}
	require.Equal(t, 30*24*time.Hour, longest, "the homepage and activation selectors offer up to 30 days")
	for _, r := range []struct{ table, reader string }{
		{"search_queries", "HomepageRepository.GetSearchPulse"},
		{"api_request_events", "HomepageRepository.GetAPIUsagePulse"},
		{"funnel_events", "ActivationAnalyticsRepository.Measure"},
		{"room_entries", "HomepageRepository.GetRoomPulse"},
	} {
		assert.Equal(t, longest, readerWindow(t, r.table, r.reader), "%s of %s", r.reader, r.table)
	}

	// /v1/data/* accepts 1h, 24h and 7d, and nothing longer.
	_, err := windowToInterval("7d")
	require.NoError(t, err)
	for _, longer := range []string{"14d", "30d", "90d", "365d"} {
		_, err := windowToInterval(longer)
		require.Error(t, err, "/v1/data/* accepts %s: RawEventSources must state it", longer)
	}
	assert.Equal(t, 7*24*time.Hour, readerWindow(t, "search_queries", "DataAnalyticsRepository"))
}

// TestRawEventSources_NothingPrunesThemByAge reads every production Go file under internal/ and
// cmd/ for SQL that deletes rows of a raw event table (or of room_entries' views) and for calls of
// a DeleteOlderThan method. A new pruner fails here until RawEventSources states its retention.
func TestRawEventSources_NothingPrunesThemByAge(t *testing.T) {
	tables := []string{"messages", "room_events"} // the views over room_entries
	for _, s := range RawEventSources {
		tables = append(tables, s.Table)
	}
	deleteRe := regexp.MustCompile(`(?i)\bDELETE\s+FROM\s+(` + strings.Join(tables, "|") + `)\b`)

	deleters := map[string]string{}
	var calls []string
	fset := token.NewFileSet()
	root := backendRoot(t)
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return err
			}
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				name := fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) == 1 {
					if star, ok := fn.Recv.List[0].Type.(*ast.StarExpr); ok {
						if id, ok := star.X.(*ast.Ident); ok {
							name = id.Name + "." + name
						}
					}
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch n := n.(type) {
					case *ast.BasicLit:
						if m := deleteRe.FindStringSubmatch(n.Value); m != nil {
							deleters[name] = strings.ToLower(m[1])
						}
					case *ast.CallExpr:
						if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "DeleteOlderThan" {
							calls = append(calls, fset.Position(n.Pos()).String())
						}
					}
					return true
				})
			}
			return nil
		})
		require.NoError(t, err)
	}

	assert.Equal(t, map[string]string{
		"SearchAnalyticsRepository.DeleteOlderThan": "search_queries",
		"ServiceCheckRepository.DeleteOlderThan":    "service_checks",
	}, deleters, "production SQL that deletes raw events")
	assert.Empty(t, calls, "a DeleteOlderThan is called: state that table's retention in RawEventSources")
	for _, s := range RawEventSources {
		assert.Equal(t, KeptIndefinitely, s.Retention, "%s: nothing prunes it", s.Table)
		assert.Empty(t, s.PrunedBy, "%s: nothing prunes it", s.Table)
	}
}

// TestRawEventSources_CleanInstallation checks the registry against a freshly migrated database,
// then pins that each distinct count is exact over its whole window: one identity seen on three
// different days of the window counts once, where a sum of per-day distinct counts would count it
// three times. The fixtures assert that they tell the two apart before the readers are asked.
func TestRawEventSources_CleanInstallation(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	week, ok := RoomStatsWindowByValue("7d")
	require.True(t, ok)
	homepage := NewHomepageRepository(pool)

	// perDaySum is what summing per-day distinct counts would report.
	perDaySum := func(t *testing.T, sql string) int {
		t.Helper()
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(SUM(n), 0)::int FROM (`+sql+`) per_day`).Scan(&n))
		return n
	}

	t.Run("every source is a table with its time column", func(t *testing.T) {
		for _, s := range RawEventSources {
			var dataType string
			err := pool.QueryRow(ctx, `
				SELECT c.data_type FROM information_schema.columns c
				  JOIN information_schema.tables tb USING (table_schema, table_name)
				 WHERE c.table_schema = 'public' AND tb.table_type = 'BASE TABLE'
				   AND c.table_name = $1 AND c.column_name = $2`, s.Table, s.TimeColumn).Scan(&dataType)
			require.NoError(t, err, "%s.%s is not a column of a table", s.Table, s.TimeColumn)
			assert.Equal(t, "timestamp with time zone", dataType, "%s.%s", s.Table, s.TimeColumn)
		}
	})

	t.Run("unique queries", func(t *testing.T) {
		searches := NewSearchAnalyticsRepository(pool)
		public := true
		for _, s := range []struct {
			q    string
			days int
		}{{"kubernetes ingress", 1}, {"kubernetes ingress", 2}, {"kubernetes ingress", 3}, {"zeromq", 2}} {
			require.NoError(t, searches.Insert(ctx, models.SearchQuery{
				Query: s.q, QueryNormalized: s.q, ResultsCount: 1, SearchMethod: "fulltext",
				DurationMs: 5, SearcherType: "agent", Page: 1, PublicScope: &public,
				SearchedAt: time.Now().Add(-time.Duration(s.days) * 24 * time.Hour),
			}))
		}
		require.Equal(t, 4, perDaySum(t, `SELECT COUNT(DISTINCT query_normalized) n FROM search_queries
			GROUP BY date_trunc('day', searched_at)`))

		summary, err := searches.GetSummary(ctx, 7)
		require.NoError(t, err)
		assert.Equal(t, 4, summary.TotalSearches)
		assert.Equal(t, 2, summary.UniqueQueries, "a query searched on three days is one query")

		// The chart and the total are both read from the same raw eligible rows.
		pulse, err := homepage.GetSearchPulse(ctx, week, 5)
		require.NoError(t, err)
		bucketed := 0
		for _, b := range pulse.Series {
			bucketed += b.Count
		}
		assert.Equal(t, 4, pulse.Eligible)
		assert.Equal(t, pulse.Eligible, bucketed)
	})

	t.Run("distinct operations", func(t *testing.T) {
		var events []APIRequestEvent
		for i, e := range []struct {
			op, family, kind string
			days             int
		}{
			{"room.message.send", APIFamilyRoom, APIOperationWrite, 1},
			{"room.message.send", APIFamilyRoom, APIOperationWrite, 2},
			{"room.message.send", APIFamilyRoom, APIOperationWrite, 3},
			{"knowledge.search", APIFamilyKnowledge, APIOperationSearch, 2},
		} {
			events = append(events, APIRequestEvent{
				RequestID: fmt.Sprintf("retention-%d", i), RouteTemplate: "/v1/" + e.op, Method: "POST",
				Operation: e.op, OperationFamily: e.family, OperationKind: e.kind, ActorType: APIActorAgent,
				StatusClass: 2, OccurredAt: time.Now().Add(-time.Duration(e.days) * 24 * time.Hour),
			})
		}
		require.NoError(t, NewAPIUsageRepository(pool).RecordRequests(ctx, events))
		require.Equal(t, 4, perDaySum(t, `SELECT COUNT(DISTINCT operation) n FROM api_request_events
			GROUP BY date_trunc('day', occurred_at)`))

		usage, err := homepage.GetAPIUsagePulse(ctx, week)
		require.NoError(t, err)
		assert.Equal(t, 4, usage.SuccessfulCalls)
		assert.Equal(t, 2, usage.RoomKnowledgeOperations, "an operation used on three days is one operation")
	})

	t.Run("rooms with conversation", func(t *testing.T) {
		var roomID string
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private)
			VALUES ('retention-room', 'Retention room', false) RETURNING id::text`).Scan(&roomID))
		for _, days := range []int{1, 2, 3} {
			_, err := pool.Exec(ctx, `INSERT INTO messages (room_id, author_type, agent_name, content, created_at)
				VALUES ($1, 'agent', 'retention_agent', 'note', NOW() - $2 * INTERVAL '1 day')`, roomID, days)
			require.NoError(t, err)
		}
		require.Equal(t, 3, perDaySum(t, `SELECT COUNT(DISTINCT room_id) n FROM messages
			GROUP BY date_trunc('day', created_at)`))

		rooms, err := homepage.GetRoomPulse(ctx, week)
		require.NoError(t, err)
		assert.Equal(t, 1, rooms.Stats.RoomsWithConversation, "a room that talked on three days is one room")
	})
}

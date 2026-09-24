package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// servedRoutes walks a router built exactly like production (pool + hub), so the
// room transport routes are mounted, and returns every "METHOD /path" it serves.
func servedRoutes(t *testing.T) map[string]bool {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	router := NewRouter(pool, hubMgr, registry)

	served := map[string]bool{}
	require.NoError(t, chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.ReplaceAll(route, "/*/", "/")
		if route != "/" {
			route = strings.TrimSuffix(route, "/")
		}
		served[method+" "+route] = true
		return nil
	}))
	require.NotEmpty(t, served)
	return served
}

// Every route the router serves has exactly one documented decision, and the
// registry names no route the router does not serve. A route added later without
// a decision fails here.
func TestRouteFamilies_CoverEveryServedRoute(t *testing.T) {
	served := servedRoutes(t)

	owner := map[string]string{}
	for _, f := range RouteFamilies {
		for _, route := range f.Routes {
			if prev, dup := owner[route]; dup {
				t.Errorf("%s is claimed by two families: %q and %q", route, prev, f.Name)
			}
			owner[route] = f.Name
		}
	}

	var undecided, phantom []string
	for route := range served {
		if _, ok := owner[route]; !ok {
			undecided = append(undecided, route)
		}
	}
	for route := range owner {
		if !served[route] {
			phantom = append(phantom, route)
		}
	}
	sort.Strings(undecided)
	sort.Strings(phantom)
	assert.Empty(t, undecided, "served routes with no keep/merge/adapt/retire decision:\n%s", strings.Join(undecided, "\n"))
	assert.Empty(t, phantom, "registry names routes the router does not serve:\n%s", strings.Join(phantom, "\n"))
}

// Each family is well formed: a known disposition, and every non-keep decision
// names the canonical destination clients move to.
func TestRouteFamilies_EveryNonKeepDecisionNamesItsCanonicalDestination(t *testing.T) {
	require.NotEmpty(t, RouteFamilies)
	names := map[string]bool{}
	for _, f := range RouteFamilies {
		assert.NotEmpty(t, f.Name)
		assert.False(t, names[f.Name], "family name %q is used twice", f.Name)
		names[f.Name] = true
		assert.NotEmpty(t, f.Routes, "family %q lists no routes", f.Name)
		switch f.Disposition {
		case DispositionKeep:
		case DispositionMerge, DispositionAdapt, DispositionRetire:
			assert.NotEmpty(t, f.Canonical, "family %q is %s but names no canonical destination", f.Name, f.Disposition)
		default:
			t.Errorf("family %q has unknown disposition %q", f.Name, f.Disposition)
		}
	}
}

// SPEC.md Part 26 publishes the same decisions: one table row per family with its
// disposition and canonical destination, and one row per non-keep route. The doc
// cannot drift from the registry the router is pinned to.
func TestRouteFamilies_PublishedInSpec(t *testing.T) {
	raw, err := os.ReadFile("../../../SPEC.md")
	require.NoError(t, err)
	spec := string(raw)
	start := strings.Index(spec, "# Part 26: Canonical Knowledge API and Route Dispositions")
	require.GreaterOrEqual(t, start, 0, "SPEC.md has no Part 26")
	part := spec[start:]
	if next := strings.Index(part[1:], "\n# Part "); next >= 0 {
		part = part[:next+1]
	}

	for _, f := range RouteFamilies {
		canonical := f.Canonical
		if canonical == "" {
			canonical = "—"
		}
		row := "| `" + f.Name + "` | " + string(f.Disposition) + " | " + canonical + " |"
		assert.Contains(t, part, row, "Part 26 family table is missing or disagrees with the registry")
		if f.Disposition == DispositionKeep {
			continue
		}
		for _, route := range f.Routes {
			assert.Contains(t, part, "| `"+route+"` | `"+f.Name+"` |",
				"Part 26 adapter mapping does not list %s under %s", route, f.Name)
		}
	}
}

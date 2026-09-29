package db

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// productionFilesContaining returns, per fragment, the non-test Go files under root whose
// source contains it.
func productionFilesContaining(t *testing.T, root string, fragments []string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for _, f := range fragments {
			if strings.Contains(string(src), f) {
				out[f] = append(out[f], filepath.ToSlash(rel))
			}
		}
		return nil
	})
	require.NoError(t, err)
	for f := range out {
		sort.Strings(out[f])
	}
	return out
}

// The legacy lifecycles are retired from every production path: cmd/api/main.go no longer
// schedules the stale-content or auto-solve job (no scanned job:StaleContentJob or
// job:AutoSolveJob, so neither keeps a job: disposition), nothing constructs their
// repositories or the forgetting service, and nothing calls the approach archive methods
// the forgetting service used. The code that remains is only its own definitions, which
// drop with the legacy tables; the registry records each retirement.
func TestLegacyLifecycles_RetiredFromProductionPaths(t *testing.T) {
	root := backendRoot(t)
	deps, err := ScanLegacySourceDependencies(root)
	require.NoError(t, err)
	keys := depKeys(deps)
	for _, job := range []string{"job:StaleContentJob", "job:AutoSolveJob"} {
		assert.NotContains(t, keys, job, "a retired lifecycle job is scheduled again")
		_, ok := LegacyDependencyDispositions[job]
		assert.False(t, ok, "%s is unscheduled, so it keeps no job: disposition", job)
	}

	callers := productionFilesContaining(t, root, []string{
		"NewStaleContentJob(", "NewAutoSolveJob(", "NewStaleContentRepository(", "NewAutoSolveRepository(",
		"NewForgettingService(", "NewForgettingServiceWithConfig(",
		".MarkForForgetting(", ".ArchiveApproach(", ".ListStaleApproaches(",
	})
	allowed := map[string][]string{
		"NewStaleContentJob(":             {"internal/jobs/stale_content.go"},
		"NewAutoSolveJob(":                {"internal/jobs/auto_solve.go"},
		"NewStaleContentRepository(":      {"internal/db/stale_content.go"},
		"NewAutoSolveRepository(":         {"internal/db/auto_solve.go"},
		"NewForgettingService(":           {"internal/services/forgetting.go"},
		"NewForgettingServiceWithConfig(": {"internal/services/forgetting.go"},
		".ArchiveApproach(":               {"internal/services/forgetting.go"},
		".ListStaleApproaches(":           {"internal/services/forgetting.go"},
	}
	for fragment, files := range callers {
		for _, f := range files {
			assert.Contains(t, allowed[fragment], f, "%s: production code outside the retired definitions uses %q", f, fragment)
		}
	}

	forgetting := LegacyDependencyDispositions["feature:forgetting"]
	assert.Equal(t, LegacyActionRetire, forgetting.Action)
	assert.True(t, forgetting.Done, "feature:forgetting is retired and verified")
	assert.Contains(t, forgetting.Note, "ForgettingService")
	for _, key := range []string{"code:internal/db/stale_content.go", "code:internal/db/auto_solve.go"} {
		d, ok := LegacyDependencyDispositions[key]
		require.True(t, ok, key)
		assert.Equal(t, LegacyActionRetire, d.Action, key)
		assert.False(t, d.Done, "%s still holds legacy SQL; it drops with the tables", key)
		assert.Contains(t, d.Note, "unscheduled", key)
	}
}

package db

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 steps 3-4 (feature:badges): milestone awarding is retired, not moved to the
// canonical model. services.BadgeService (first_solve, ten_solves, hundred_upvotes,
// first_answer_accepted) has had no production caller since it was written, and its rules
// count solved problems and accepted answers the canonical model does not have. What stays
// is the history: earned badge rows, served unchanged (TestBadgeRoutes_ServeEarnedBadges
// AsHistoryAcrossTheCutover). This pins the retirement: no production Go file constructs the
// service or runs its milestone check, and the registry records exactly that decision.
func TestLegacyBadgeMilestones_RetiredWithoutAProductionCaller(t *testing.T) {
	root := backendRoot(t)
	var callers []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if filepath.ToSlash(rel) == "internal/services/badges.go" {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if text := string(src); strings.Contains(text, "NewBadgeService(") || strings.Contains(text, "CheckAndAwardBadges(") {
			callers = append(callers, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, callers, "legacy milestone awarding is retired (feature:badges); award from canonical facts only through a new, reviewed rule")

	d, ok := LegacyDependencyDispositions["feature:badges"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionRetire, d.Action)
	assert.True(t, d.Done, "feature:badges is retired and verified")
}

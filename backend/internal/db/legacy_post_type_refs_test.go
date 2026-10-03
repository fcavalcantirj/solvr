package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Go code that names a legacy post type through a constant or a bare string literal is
// found: the SQL scan cannot see a type passed as a bound parameter or compared in Go, and
// every such file silently changes behavior once check:posts.posts_type_check narrows.
func TestScanLegacyPostTypeReferences_FindsConstantsAndLiterals(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/db/param.go", "package db\n\nvar ideaType = string(models.PostTypeIdea)\n")
	writeSource(t, root, "internal/api/handlers/branch.go", "package handlers\n\nfunc f(t string) int {\n\tswitch t {\n\tcase \"problem\":\n\t\treturn 1\n\t}\n\treturn 0\n}\n")
	writeSource(t, root, "internal/api/router.go", "package api\n\nvar route = adapter(list, `question`)\n")
	writeSource(t, root, "internal/models/post.go", "package models\n\nconst (\n\tPostTypeProblem PostType = \"problem\"\n)\n\nfunc ok(t PostType) bool { return t == PostTypeProblem }\n")
	// Owned by the file's legacy SQL entry: it drops or changes with that code.
	writeSource(t, root, "internal/db/legacy.go", "package db\n\nconst q = `SELECT id FROM answers`\n\nfunc g(p Post) bool { return p.Type == models.PostTypeQuestion }\n")
	// Not a reference: tags, comments, prose, plurals, other words, tests, imports, vendor.
	writeSource(t, root, "internal/models/tagged.go", "package models\n\n// Type is \"problem\" or \"idea\".\ntype X struct {\n\tQ string `json:\"question\"`\n}\n\nvar msg = \"problem not found\"\nvar plural = \"problems\"\nvar other = \"post\"\n")
	writeSource(t, root, "internal/db/imports.go", "package db\n\nimport idea \"example.com/idea\"\n\nvar _ = idea.X\n")
	writeSource(t, root, "internal/db/types_test.go", "package db\n\nvar tt = models.PostTypeProblem\nvar ss = \"idea\"\n")
	writeSource(t, root, "vendor/x/x.go", "package x\n\nvar v = \"problem\"\n")

	deps, err := ScanLegacyPostTypeReferences(root)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"typeref:internal/api/handlers/branch.go",
		"typeref:internal/api/router.go",
		"typeref:internal/db/legacy.go",
		"typeref:internal/db/param.go",
		"typeref:internal/models/post.go",
	}, depKeys(deps))
	byKey := map[string]LegacyDependency{}
	for _, d := range deps {
		byKey[d.Key] = d
		assert.Equal(t, "typeref", d.Kind)
	}
	assert.Equal(t, "idea at line 3", byKey["typeref:internal/db/param.go"].Detail)
	assert.Equal(t, "problem at line 5", byKey["typeref:internal/api/handlers/branch.go"].Detail)
	assert.Equal(t, "question at line 3", byKey["typeref:internal/api/router.go"].Detail)
	assert.Equal(t, "problem at lines 4,7", byKey["typeref:internal/models/post.go"].Detail)
	assert.Equal(t, "code:internal/db/legacy.go", byKey["typeref:internal/db/legacy.go"].Owner)
	assert.Empty(t, byKey["typeref:internal/db/param.go"].Owner)
}

// A file that does not parse is an error, not a silent gap in the inventory.
func TestScanLegacyPostTypeReferences_UnparsableFileIsAnError(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "internal/db/broken.go", "package db\n\nfunc {\n")
	_, err := ScanLegacyPostTypeReferences(root)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal/db/broken.go")
}

// Every Go file in the real tree that names a legacy post type has a disposition (its own,
// or its legacy SQL file's), and the registry names none that is gone. A pending one blocks
// schema cleanup through LegacySchemaCleanupGate like any other entry.
func TestLegacyDependencyRegistry_CoversThePostTypeReferences(t *testing.T) {
	deps, err := ScanLegacyPostTypeReferences(backendRoot(t))
	require.NoError(t, err)
	keys := depKeys(deps)
	// Known references (the scan is not vacuous): a canonical counter passing the type as a
	// parameter, a canonical route branching on it, and an unmounted per-type statistics handler.
	for _, k := range []string{
		"typeref:internal/db/profile_stats_canonical.go",
		"typeref:internal/api/handlers/posts.go",
		"typeref:internal/api/handlers/stats.go",
		"typeref:internal/models/post.go",
	} {
		assert.Contains(t, keys, k)
	}
	assertRegistryCovers(t, deps, map[string]bool{"typeref": true})
	for _, d := range deps {
		if d.Owner != "" {
			_, ok := LegacyDependencyDispositions[d.Owner]
			assert.True(t, ok, "%s is owned by %s, which has no disposition", d.Key, d.Owner)
		}
	}
}

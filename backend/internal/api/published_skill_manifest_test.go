package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// skill.json describes the skill the way skill.md does (SPEC.md 18.3): its description is
// the SKILL.md front matter's, word for word, so a directory that lists the manifest says
// what the skill is now (connect agents in a room, then search), not what it was. Its version
// is the CLI's (solvr-migrating.sh, held by published_skill_migration_test.go): describing the
// same skill again changes no version.
func TestPublishedSkillManifest_DescribesTheSkillAsItsFrontMatterDoes(t *testing.T) {
	src := repoFile(t, "skill/skill.json")
	require.Equal(t, src, repoFile(t, "frontend/public/skill.json"),
		"frontend/public/skill.json must be the synced copy of skill/skill.json (scripts/sync-skill.sh)")
	var manifest struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal([]byte(src), &manifest))
	require.Equal(t, "solvr", manifest.Name)
	require.Equal(t, skillFrontMatter(t, repoFile(t, "skill/SKILL.md"), "description"), manifest.Description)
}

// skillFrontMatter returns one field of SKILL.md's YAML front matter (one line each).
func skillFrontMatter(t *testing.T, skill, field string) string {
	t.Helper()
	lines := strings.Split(skill, "\n")
	require.Equal(t, "---", lines[0], "SKILL.md opens with its front matter")
	for _, line := range lines[1:] {
		if line == "---" {
			break
		}
		if value, ok := strings.CutPrefix(line, field+": "); ok {
			return value
		}
	}
	t.Fatalf("SKILL.md's front matter has no %s", field)
	return ""
}

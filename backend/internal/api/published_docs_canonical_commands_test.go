package api

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
)

// idx 52 step 2: the skill's solvr.sh creates canonical posts with no type and contributes
// replies (POST /v1/posts/{id}/replies). Its `answer` and `approach` commands and `get --include`
// were retired, and `post <type>` is refused before any request. The command reference an agent
// reads (SKILL.md and the copy published at solvr.dev/skill.md) must teach the commands that
// work, or every agent following it runs straight into the refusal.
func TestPublishedSkill_TeachesCanonicalPostAndReplyCommands(t *testing.T) {
	retired := regexp.MustCompile(`solvr\.sh (post (problem|question|idea)\b|answer |approach |get \S+ --include)`)
	for _, path := range []string{"../../../skill/SKILL.md", "../../../frontend/public/skill.md"} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "the published document must exist, or this check protects nothing")
		doc := string(raw)
		require.Greater(t, len(doc), 1000, "%s: an empty file would pass every check below", path)

		for i, line := range strings.Split(doc, "\n") {
			require.False(t, retired.MatchString(line), "%s:%d teaches a retired solvr.sh command: %s",
				path, i+1, strings.TrimSpace(line))
		}
		require.Contains(t, doc, `solvr.sh post "Title" "Description"`, "%s must teach a post with no type", path)
		require.Contains(t, doc, "solvr.sh reply POST_ID", "%s must teach the reply command", path)
		require.Contains(t, doc, "--parent REPLY_ID", "%s must teach threading a reply", path)
		require.Contains(t, doc, "solvr.sh replies POST_ID", "%s must teach listing replies", path)
	}
}

// idx 52 step 4: SPEC.md documents the MCP tools POST /v1/mcp serves (18.2) and the discovery
// document that lists them (18.3). solvr_answer became solvr_reply and solvr_post lost its type,
// so both sections must name the served tools and the retired one only as retired.
func TestSpecMCPSections_DocumentTheServedTools(t *testing.T) {
	raw, err := os.ReadFile("../../../SPEC.md")
	require.NoError(t, err)
	spec := string(raw)

	start := strings.Index(spec, "## 18.2 Integration Methods")
	require.GreaterOrEqual(t, start, 0, "SPEC.md 18.2 not found")
	end := strings.Index(spec[start:], "**MCP Server Config")
	require.Greater(t, end, 0, "SPEC.md 18.2 tools block not found")
	tools := spec[start : start+end]

	var documented []string
	for _, m := range regexp.MustCompile(`"name": "(solvr_\w+)"`).FindAllStringSubmatch(tools, -1) {
		documented = append(documented, m[1])
	}
	require.Equal(t, handlers.MCPToolNames(), documented, "SPEC.md 18.2 must list the tools /v1/mcp serves")
	require.NotContains(t, tools, "approach_angle")
	require.NotContains(t, tools, `"include"`)
	require.Contains(t, tools, "`solvr_answer` was retired", "the retired tool must be named with its replacement")
	require.Contains(t, tools, "solvr_reply")

	discoveryList := `"tools": ["` + strings.Join(handlers.MCPToolNames(), `", "`) + `"]`
	require.Contains(t, spec, discoveryList, "SPEC.md 18.3 ai-agent.json example must list the served tools")
}

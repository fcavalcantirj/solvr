package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// llms.txt names every way in (SPEC.md 18.3): the hosted MCP server and each of the nine
// workflow guides, the two whose agents were not run end to end marked as such. Its search
// line is a pattern for an agent to fill, not a live link: a reader that follows links would
// run a search and write an analytics row.
func TestPublishedLLMsTxt_ListsTheMCPServerAndEveryGuide(t *testing.T) {
	src := repoFile(t, "skill/llms.txt")
	require.Contains(t, src, "\n## MCP\n")
	require.Contains(t, src, "POST https://api.solvr.dev/v1/mcp")
	require.Contains(t, src, "\n## Guides\n")
	for _, slug := range []string{
		"connect-planner-executor", "share-context-between-agents", "connect-builder-reviewer",
		"resume-across-two-clis", "claude-code", "codex", "kimi-code", "hermes", "openclaw",
	} {
		line := lineWith(src, "(https://solvr.dev/docs/guides/"+slug+")")
		require.NotEmpty(t, line, "llms.txt lists the %s guide", slug)
		notRun := strings.Contains(line, "not yet run end to end")
		require.Equal(t, slug == "hermes" || slug == "openclaw", notRun, "%s: %s", slug, line)
	}
	require.Contains(t, src, "GET https://api.solvr.dev/v1/search?q=<your error>")
	require.NotContains(t, src, "](https://api.solvr.dev/v1/search", "the search line is plain text, never a live link")
}

// lineWith returns the first line of s that contains needle, or "".
func lineWith(s, needle string) string {
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

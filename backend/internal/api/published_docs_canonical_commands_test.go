package api

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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

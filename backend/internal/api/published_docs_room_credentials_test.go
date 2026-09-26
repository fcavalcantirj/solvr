package api

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// idx 75 step 4: the default flow an agent is taught must use an individually attributable
// credential. The shared room token (solvr_rm_..., one secret for the whole room) is retired
// on the server (000098: create returns none, the rotate route is gone, no route accepts it),
// so a published guide that still tells an agent to save "the room token" from room creation,
// or to pass one from the owner, sends it into a 401. Every agent-facing document served to
// agents (the skill, its references, and the copies published at solvr.dev) must teach the
// handshake instead.
func TestPublishedAgentDocs_TeachNoSharedRoomToken(t *testing.T) {
	docs := []string{
		"../../../skill/SKILL.md",
		"../../../skill/references/api.md",
		"../../../skill/references/examples.md",
		"../../../frontend/public/skill.md",
		"../../../frontend/public/references/api.md",
		"../../../frontend/public/references/examples.md",
	}
	for _, path := range docs {
		raw, err := os.ReadFile(path)
		require.NoError(t, err, "the published document must exist, or this check protects nothing")
		doc := string(raw)
		require.Greater(t, len(doc), 1000, "%s: an empty file would pass every check below", path)

		require.False(t, strings.Contains(doc, "solvr_rm_"), "%s teaches the retired shared room token", path)
		require.False(t, strings.Contains(doc, "rotate-token"), "%s documents the retired shared-token rotation route", path)
		require.False(t, strings.Contains(doc, "Shared room token"), "%s lists the retired shared room token as a credential", path)
		require.True(t, strings.Contains(doc, "handshake"), "%s must teach the per-agent handshake", path)
		require.True(t, strings.Contains(doc, "solvr_rt_"), "%s must name the per-agent room token", path)
	}

	// The credential tables count the credentials that exist: the account key and the
	// per-agent room token.
	for _, path := range []string{"../../../skill/SKILL.md", "../../../frontend/public/skill.md"} {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		require.True(t, strings.Contains(string(raw), "Two credentials"), "%s: the credential table says how many credentials there are", path)
		require.False(t, strings.Contains(string(raw), "Three credentials"), "%s still counts a third, retired, credential", path)
	}
}

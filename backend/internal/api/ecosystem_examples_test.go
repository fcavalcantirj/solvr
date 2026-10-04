package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/growth"
)

// Optional ecosystem distribution (spec.json idx 91). Every first-party package and the agent
// skill link straight to the no-install HTTPS connection flow and the public demonstration room,
// and say they are optional, so no ecosystem becomes a prerequisite for using Solvr. The HTTPS
// flow itself is proven in router_connect_http_only_test.go; this file holds the documents to it.

// ecosystemDocs are the skill, MCP, CLI and SDK documents, relative to the repository root.
var ecosystemDocs = []string{
	"skill/SKILL.md",
	"frontend/public/skill.md",
	"mcp-server/README.md",
	"packages/cli/README.md",
	"packages/sdk-python/README.md",
	"packages/sdk-ts/README.md",
	"packages/sdk-go/README.md",
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", rel))
	require.NoError(t, err, rel)
	return string(raw)
}

func TestEcosystemDocs_LinkTheNoInstallFlowAndTheDemo(t *testing.T) {
	demo := "https://solvr.dev/rooms/" + growth.PublicDemoRoomSlug
	for _, rel := range ecosystemDocs {
		t.Run(rel, func(t *testing.T) {
			doc := repoFile(t, rel)
			for _, want := range []string{
				"## No install needed: connect over HTTPS",
				"https://solvr.dev/connect",
				"https://api.solvr.dev/v1/connect",
				demo,
				"is optional",
				"never required",
			} {
				assert.Contains(t, doc, want)
			}
			// The baseline comes before any install step.
			if i := strings.Index(doc, "## Installation"); i >= 0 {
				assert.Less(t, strings.Index(doc, "## No install needed"), i, "the HTTPS baseline must come first")
			}
		})
	}
}

func TestEcosystemDocs_DemoIsTheHomepageExampleRoom(t *testing.T) {
	// Since v1.3.7 the hero's "Watch an example" jumps to the homepage example section, which
	// quotes the room the API names (GET /v1/homepage/example): the documents' demo is that room.
	assert.Equal(t, handlers.DefaultCollabExampleRoomSlug, growth.PublicDemoRoomSlug)
	hero := repoFile(t, "frontend/components/hero-section.tsx")
	assert.Contains(t, hero, `href="#example"`, "the hero points at the example section")
	_, err := os.Stat(filepath.Join("..", "..", "..", "frontend", "app", "connect", "page.tsx"))
	assert.NoError(t, err, "the /connect page the documents link to must exist")
}

func TestEcosystemDocs_TheLinkedContractAnswers(t *testing.T) {
	ts, _, _, cleanup := setupOperatorTestServer(t)
	defer cleanup()
	resp, raw := call(t, http.MethodGet, ts.URL+"/v1/connect", nil, "")
	require.Equal(t, http.StatusOK, resp.StatusCode, raw)
	assert.Contains(t, raw, "flow_id")
}

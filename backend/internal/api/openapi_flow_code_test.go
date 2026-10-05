package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// The OpenAPI document says what changed with the flow code (SPEC.md 25.6 and 25.7): the
// flow parameter and the link of GET /v1/connect, the web server's step and request_mode on
// the funnel ingest, the three channels of the contract, and the rule POST /v1/rooms applies
// to flow_id.
func TestOpenAPI_DocumentsTheFlowCode(t *testing.T) {
	spec := servedSpec(t)
	describes := func(what, text string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			require.Contains(t, text, want, "%s must say %q", what, want)
		}
	}
	description := func(method, path string) string {
		t.Helper()
		text, _ := operation(t, spec, method, path)["description"].(string)
		require.NotEmpty(t, text, "%s %s has no description", method, path)
		return text
	}

	describes("GET /connect", description("get", "/connect"),
		"selected.flow_id", "8 characters", models.FlowCodeAlphabet, "skill.md?f=", "flow query parameter", "ignored",
		// A page rendered on a server starts no flow.
		"flow=none", "no selected.flow_id", "plain link https://solvr.dev/skill.md", "rendered on a server")
	describes("POST /analytics/funnel", description("post", "/analytics/funnel"),
		models.FunnelSkillFetched, "flow_id", "request_mode", "Sec-Fetch-Mode", "entry_surface",
		models.FunnelSurfaceAgentFetch, models.FunnelSurfaceBrowserVisit,
		// A bot is not an agent: the third surface, read from the reported user agent.
		"user_agent", "User-Agent", "200 characters", models.FunnelSurfaceBotFetch, "link-preview", "crawler")

	// The examples endpoint says what it serves. Its summary used to describe a list of rooms.
	examples := operation(t, spec, "get", "/connect/examples")
	require.Equal(t, "Read the three example connect sentences", examples["summary"])
	describes("GET /connect/examples", description("get", "/connect/examples"),
		"example intent", "no flow_id", "https://solvr.dev/skill.md")
	for _, op := range []string{"summary", "description"} {
		text, _ := examples[op].(string)
		require.NotContains(t, strings.ToLower(text), "rooms that started", "GET /connect/examples %s", op)
		require.NotContains(t, strings.ToLower(text), "list public rooms", "GET /connect/examples %s", op)
	}
	describes("GET /analytics/funnel/contract", description("get", "/analytics/funnel/contract"),
		models.FunnelSourceBrowser, models.FunnelSourceWebServer, models.FunnelSourceServer)

	flowID := at(t, spec, "components", "schemas", "CreateRoomRequest", "properties", "flow_id").(map[string]interface{})
	require.Equal(t, "string", flowID["type"])
	text, _ := flowID["description"].(string)
	describes("CreateRoomRequest.flow_id", text, "?f=", "earlier funnel step", "ignored", "never refused",
		// A bot's fetch or a person's visit of the skill link does not make a code known.
		"link preview", "crawler", "does not count")
	require.False(t, strings.Contains(text, "prompt carried."), "the former wording is gone")
}

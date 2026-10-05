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
		"selected.flow_id", "8 characters", models.FlowCodeAlphabet, "skill.md?f=", "flow query parameter", "ignored")
	describes("POST /analytics/funnel", description("post", "/analytics/funnel"),
		models.FunnelSkillFetched, "flow_id", "request_mode", "Sec-Fetch-Mode", "entry_surface",
		models.FunnelSurfaceAgentFetch, models.FunnelSurfaceBrowserVisit)
	describes("GET /analytics/funnel/contract", description("get", "/analytics/funnel/contract"),
		models.FunnelSourceBrowser, models.FunnelSourceWebServer, models.FunnelSourceServer)

	flowID := at(t, spec, "components", "schemas", "CreateRoomRequest", "properties", "flow_id").(map[string]interface{})
	require.Equal(t, "string", flowID["type"])
	text, _ := flowID["description"].(string)
	describes("CreateRoomRequest.flow_id", text, "?f=", "earlier funnel step", "ignored", "never refused")
	require.False(t, strings.Contains(text, "prompt carried."), "the former wording is gone")
}
